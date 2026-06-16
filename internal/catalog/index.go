package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/home-beekeeper/beekeeper/internal/platform"
	mmap "github.com/edsrzf/mmap-go"
)

// Binary index format (RESEARCH Pattern 2). All multi-byte integers are
// little-endian. The whole file is always mmapped at offset 0 to sidestep the
// Windows allocation-granularity alignment pitfall (Pitfall 1).
//
//	[ 16-byte header ]
//	   [4]  magic   = 0x42454549 ("BEEI")
//	   [4]  version = uint32 (1)
//	   [4]  count   = uint32 record count
//	   [4]  reserved = 0
//	[ count * 48-byte records ]   (sorted ascending by Key)
//	   [32] Key        = sha256(ecosystem + "::" + lower(package))[:32]
//	   [8]  DataOffset = uint64 (relative to start of data section)
//	   [8]  DataLength = uint64
//	[ data section ]              (concatenated entry JSON blobs)
const (
	indexMagic      uint32 = 0x42454549 // "BEEI"
	indexVersion    uint32 = 1
	headerSize             = 16
	keySize                = 32
	recordSize             = keySize + 8 + 8 // 48 bytes
	offDataOffset          = keySize         // within a record
	offDataLength          = keySize + 8     // within a record
)

// indexKey derives the fixed-width lookup key for an (ecosystem, package) pair.
// Package names are lowercased for case-insensitive matching; this MUST match
// between BuildIndex and Lookup.
func indexKey(ecosystem, pkg string) [keySize]byte {
	sum := sha256.Sum256([]byte(ecosystem + "::" + strings.ToLower(pkg)))
	var k [keySize]byte
	copy(k[:], sum[:keySize])
	return k
}

// indexRecord is the in-memory form used while building the index.
type indexRecord struct {
	key  [keySize]byte
	data []byte // JSON blob for the entry
}

// BuildIndex writes a sorted binary index file at path from entries. Keys are
// deduplicated (last entry for a given (ecosystem, package) wins) and records
// are sorted ascending by key to permit binary search at read time. The file
// is written to a temp file and atomically renamed into place so a partial
// write never leaves a corrupt index where a reader expects a valid one.
func BuildIndex(path string, entries []Entry) error {
	// Deduplicate by key, last wins, preserving the order of last occurrence.
	byKey := make(map[[keySize]byte][]byte, len(entries))
	for i := range entries {
		k := indexKey(entries[i].Ecosystem, entries[i].Package)
		blob, err := json.Marshal(entries[i])
		if err != nil {
			return fmt.Errorf("marshal entry %q: %w", entries[i].ID, err)
		}
		byKey[k] = blob
	}

	records := make([]indexRecord, 0, len(byKey))
	for k, blob := range byKey {
		records = append(records, indexRecord{key: k, data: blob})
	}
	sort.Slice(records, func(i, j int) bool {
		return bytes.Compare(records[i].key[:], records[j].key[:]) < 0
	})

	count := uint32(len(records))

	// Assemble the full file in a buffer, then write atomically.
	var buf bytes.Buffer

	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], indexMagic)
	binary.LittleEndian.PutUint32(header[4:8], indexVersion)
	binary.LittleEndian.PutUint32(header[8:12], count)
	binary.LittleEndian.PutUint32(header[12:16], 0) // reserved
	buf.Write(header)

	// Data offsets are relative to the start of the data section.
	var dataOff uint64
	recBuf := make([]byte, recordSize)
	var dataSection bytes.Buffer
	for i := range records {
		dataLen := uint64(len(records[i].data))
		copy(recBuf[0:keySize], records[i].key[:])
		binary.LittleEndian.PutUint64(recBuf[offDataOffset:offDataOffset+8], dataOff)
		binary.LittleEndian.PutUint64(recBuf[offDataLength:offDataLength+8], dataLen)
		buf.Write(recBuf)

		dataSection.Write(records[i].data)
		dataOff += dataLen
	}
	buf.Write(dataSection.Bytes())

	if err := writeFileAtomic(path, buf.Bytes()); err != nil {
		return fmt.Errorf("write index %q: %w", path, err)
	}
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory then renames
// it over path, so readers never observe a partially written index.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// SEC (remediation 260615, #2/#3): catalog index/JSON files live under the
	// self-protected StateDir and drive block decisions — the on-disk `Signed`
	// byte is trusted by corroboration. Enforce owner-only so a non-owner local
	// process cannot tamper the index to flip a block into an allow (or forge a
	// block). The local overlay already hardened its own files; doing it here in
	// the shared writer makes it structural for every catalog file — notably
	// bumblebee.idx, the primary decision input, which previously inherited only
	// the process umask.
	return platform.SetOwnerOnly(path)
}

// Index is a read-only, memory-mapped view of a binary index file. It performs
// O(log n) lookups without reading or parsing the source catalog JSON
// (HOOK-02). Callers must Close it to unmap.
type Index struct {
	f     *os.File
	mm    mmap.MMap
	count int
	// recOff is the byte offset of the first record (== headerSize).
	recOff int
	// dataOff is the byte offset of the data section.
	dataOff int
}

// OpenIndex memory-maps an existing index file read-only and validates its
// header. A wrong magic, unsupported version, or truncated header/records
// region yields an error so the hook handler can fail closed rather than read
// arbitrary memory.
func OpenIndex(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}

	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap index: %w", err)
	}

	if len(mm) < headerSize {
		mm.Unmap()
		f.Close()
		return nil, fmt.Errorf("index too small: %d bytes", len(mm))
	}

	magic := binary.LittleEndian.Uint32(mm[0:4])
	if magic != indexMagic {
		mm.Unmap()
		f.Close()
		return nil, fmt.Errorf("bad index magic: 0x%08x (want 0x%08x)", magic, indexMagic)
	}
	version := binary.LittleEndian.Uint32(mm[4:8])
	if version != indexVersion {
		mm.Unmap()
		f.Close()
		return nil, fmt.Errorf("unsupported index version: %d (want %d)", version, indexVersion)
	}
	count := int(binary.LittleEndian.Uint32(mm[8:12]))

	// SEC (remediation 260615): guard against a crafted count that would overflow
	// the records-region size computation on a 32-bit build and slip past the
	// truncation check below with a too-small mmap. recordSize > 0, so a valid
	// index always satisfies count <= (len-headerSize)/recordSize; anything larger
	// is malformed and must fail closed (no match → handler blocks for bumblebee).
	if count < 0 || count > (len(mm)-headerSize)/recordSize {
		mm.Unmap()
		f.Close()
		return nil, fmt.Errorf("index count %d implausible for %d-byte file", count, len(mm))
	}

	recOff := headerSize
	dataOff := headerSize + count*recordSize
	if len(mm) < dataOff {
		mm.Unmap()
		f.Close()
		return nil, fmt.Errorf("index truncated: have %d bytes, need at least %d for %d records", len(mm), dataOff, count)
	}

	return &Index{
		f:       f,
		mm:      mm,
		count:   count,
		recOff:  recOff,
		dataOff: dataOff,
	}, nil
}

// recordKey returns the key bytes of record i (0-based) as a sub-slice of the
// mmap. Callers must not retain it past Close.
func (idx *Index) recordKey(i int) []byte {
	base := idx.recOff + i*recordSize
	return idx.mm[base : base+keySize]
}

// recordData returns the JSON data slice referenced by record i.
func (idx *Index) recordData(i int) ([]byte, error) {
	base := idx.recOff + i*recordSize
	off := binary.LittleEndian.Uint64(idx.mm[base+offDataOffset : base+offDataOffset+8])
	length := binary.LittleEndian.Uint64(idx.mm[base+offDataLength : base+offDataLength+8])
	start := uint64(idx.dataOff) + off
	end := start + length
	if end > uint64(len(idx.mm)) || start > end {
		return nil, fmt.Errorf("index record %d data range [%d:%d] out of bounds (len %d)", i, start, end, len(idx.mm))
	}
	return idx.mm[start:end], nil
}

// Lookup returns the catalog Entry for an exact (ecosystem, package) match, or
// ok=false if absent. It uses binary search over the sorted record array; no
// JSON file is read — only the matched entry blob inside the mmap is unmarshaled.
func (idx *Index) Lookup(ecosystem, pkg string) (Entry, bool) {
	key := indexKey(ecosystem, pkg)

	i := sort.Search(idx.count, func(i int) bool {
		return bytes.Compare(idx.recordKey(i), key[:]) >= 0
	})
	if i >= idx.count {
		return Entry{}, false
	}
	if !bytes.Equal(idx.recordKey(i), key[:]) {
		return Entry{}, false
	}

	data, err := idx.recordData(i)
	if err != nil {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, false
	}
	return e, true
}

// Count returns the number of records (unique keys) in the index.
func (idx *Index) Count() int { return idx.count }

// Close unmaps the index and closes the underlying file.
func (idx *Index) Close() error {
	var firstErr error
	if idx.mm != nil {
		if err := idx.mm.Unmap(); err != nil {
			firstErr = err
		}
		idx.mm = nil
	}
	if idx.f != nil {
		if err := idx.f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		idx.f = nil
	}
	return firstErr
}

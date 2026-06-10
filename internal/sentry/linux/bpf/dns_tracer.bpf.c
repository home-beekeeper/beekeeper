//go:build ignore

// dns_tracer.bpf.c — DNS query ingestion for the Beekeeper Sentry (Phase 20,
// SENT-11, OPTIONAL stretch). A kprobe on udp_sendmsg/tcp_sendmsg filtered to
// destination port 53 copies the raw DNS question bytes (length-prefixed wire
// format) into the event; the QNAME is decoded in Go (parseEvent) to keep the
// in-kernel path verifier-friendly. Closes the DNS-TXT tunnelling gap that the
// TCP-connect-only network source cannot see (analysis F1).
//
// CI-VALIDATED: this source is compiled to bytecode in the Ubuntu eBPF CI matrix
// (kernels 5.4 / 5.15), never at runtime, and never committed. The committed
// bpf_beekeeper_dns_bpfel.go is a fail-closed loader stub. The msg_iter.iov
// access targets pre-6.4 kernels (the CI matrix); on 6.4+ the field is renamed
// __iov and would need a CO-RE variant.

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define DNS_QBUF 256

struct dns_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u16 dport;       /* network byte order, == htons(53) */
    __u8  pad[2];
    __u64 ktime_ns;
    __u8  qbuf[DNS_QBUF]; /* raw DNS message bytes; QNAME decoded in Go */
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} dns_events SEC(".maps");

// handle_dns is shared by the udp_sendmsg and tcp_sendmsg kprobes. sk is PARM1,
// msg is PARM2 in both kernel functions.
static __always_inline int handle_dns(struct sock *sk, struct msghdr *msg)
{
    __u16 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    if (dport != bpf_htons(53))
        return 0;

    struct dns_event *e = bpf_ringbuf_reserve(&dns_events, sizeof(*e), 0);
    if (!e)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = (__u32)BPF_CORE_READ(task, real_parent, tgid);
    e->uid = (__u32)bpf_get_current_uid_gid();
    e->ktime_ns = bpf_ktime_get_ns();
    e->dport = dport;
    e->pad[0] = 0;
    e->pad[1] = 0;
    __builtin_memset(e->qbuf, 0, sizeof(e->qbuf));

    // Copy the outgoing message payload (DNS query) from the first iovec. The
    // CI matrix targets pre-6.4 kernels where the field is msg_iter.iov; Go
    // decodes the length-prefixed QNAME from qbuf (skipping the 12-byte DNS
    // header for UDP).
    const void *base = BPF_CORE_READ(msg, msg_iter.iov, iov_base);
    if (base)
        bpf_probe_read_user(e->qbuf, sizeof(e->qbuf), base);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("kprobe/udp_sendmsg")
int kprobe__udp_sendmsg(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    return handle_dns(sk, msg);
}

SEC("kprobe/tcp_sendmsg")
int kprobe__tcp_sendmsg(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    return handle_dns(sk, msg);
}

char __license[] SEC("license") = "GPL";

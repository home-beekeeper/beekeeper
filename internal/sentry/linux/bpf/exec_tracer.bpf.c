//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct process_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u8  exe[256];
    __u8  cmdline[512];
    __u64 ktime_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} process_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} process_events_perf SEC(".maps");

SEC("tracepoint/sched/sched_process_exec")
int tracepoint__sched__sched_process_exec(struct trace_event_raw_sched_process_exec *ctx)
{
    struct process_event *e;
    e = bpf_ringbuf_reserve(&process_events, sizeof(*e), 0);
    if (!e)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = (__u32)BPF_CORE_READ(task, real_parent, tgid);
    e->uid = (__u32)bpf_get_current_uid_gid();
    e->ktime_ns = bpf_ktime_get_ns();

    unsigned int fname_off = BPF_CORE_READ(ctx, __data_loc_filename) & 0xFFFF;
    bpf_probe_read_str(e->exe, sizeof(e->exe), (void *)ctx + fname_off);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char __license[] SEC("license") = "GPL";

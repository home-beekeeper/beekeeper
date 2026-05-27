//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct network_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u32 saddr;
    __u32 daddr;
    __u16 dport;
    __u16 sport;
    __u8  is_ipv6;
    __u8  pad[3];
    __u8  saddr6[16];
    __u8  daddr6[16];
    __u64 ktime_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} network_events SEC(".maps");

SEC("kprobe/tcp_connect")
int kprobe__tcp_connect(struct pt_regs *ctx)
{
    struct network_event *e;
    e = bpf_ringbuf_reserve(&network_events, sizeof(*e), 0);
    if (!e)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = (__u32)BPF_CORE_READ(task, real_parent, tgid);
    e->uid = (__u32)bpf_get_current_uid_gid();
    e->ktime_ns = bpf_ktime_get_ns();

    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    __u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
    e->is_ipv6 = (family == 10) ? 1 : 0;  /* AF_INET6 = 10 */

    if (!e->is_ipv6) {
        e->daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
        e->saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
        e->dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
        e->sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    } else {
        BPF_CORE_READ_INTO(e->daddr6, sk, __sk_common.skc_v6_daddr.in6_u.u6_addr8);
        BPF_CORE_READ_INTO(e->saddr6, sk, __sk_common.skc_v6_rcv_saddr.in6_u.u6_addr8);
        e->dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    }

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char __license[] SEC("license") = "GPL";

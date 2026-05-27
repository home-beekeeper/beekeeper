//go:build linux

package linux

//go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperExec ./bpf/exec_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers
//go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperNet ./bpf/network_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers

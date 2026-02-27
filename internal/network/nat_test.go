package network

import (
	"reflect"
	"testing"
)

func TestMasqueradeArgs(t *testing.T) {
	want := []string{
		"-t", "nat",
		"-A", "POSTROUTING",
		"-s", "172.17.0.0/16",
		"!", "-o", "minidocker0",
		"-j", "MASQUERADE",
	}
	got := masqueradeArgs("minidocker0")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("masqueradeArgs() = %v, want %v", got, want)
	}
}

func TestPortMappingRules(t *testing.T) {
	rules := portMappingRules("172.17.0.42", PortMapping{HostPort: 8080, ContainerPort: 80})
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}

	preroute := rules[0]
	if preroute.table != "nat" || preroute.chain != "PREROUTING" {
		t.Fatalf("first rule = %+v, want PREROUTING DNAT", preroute)
	}
	if !containsAll(preroute.args, "--dport", "8080", "--to-destination", "172.17.0.42:80") {
		t.Fatalf("PREROUTING args = %v", preroute.args)
	}

	output := rules[1]
	if output.chain != "OUTPUT" {
		t.Fatalf("second rule chain = %q, want OUTPUT", output.chain)
	}

	forward := rules[2]
	if forward.table != "filter" || forward.chain != "FORWARD" {
		t.Fatalf("third rule = %+v, want FORWARD accept", forward)
	}
	if !containsAll(forward.args, "-d", "172.17.0.42", "--dport", "80") {
		t.Fatalf("FORWARD args = %v", forward.args)
	}
}

func containsAll(slice []string, values ...string) bool {
	for _, v := range values {
		found := false
		for _, s := range slice {
			if s == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

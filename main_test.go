package main

import "testing"

func parsedAs(t *testing.T, inputHostname, outputName, outputType string) {
	name, typ := parseConsulAddress(inputHostname)
	if name != outputName || typ != outputType {
		t.Errorf("error parsing %s: expected %s %s got %s %s", inputHostname, outputName, outputType, name, typ)
	}
}

func Test_parseConsulAddress(t *testing.T) {
	parsedAs(t, "foobar.service.consul", "foobar", "")
	parsedAs(t, "_foobar._http.service.consul", "foobar", "http")
	parsedAs(t, "_foobar._rpc.service.consul", "foobar", "rpc")

	parsedAs(t, "foobar.service.site.consul", "foobar", "")
	parsedAs(t, "_foobar._http.service.site.consul", "foobar", "http")
	parsedAs(t, "_foobar._rpc.service.site.consul", "foobar", "rpc")
}

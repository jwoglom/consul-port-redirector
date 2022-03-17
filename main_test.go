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
	parsedAs(t, "http.foobar.service.consul", "foobar", "http")
	parsedAs(t, "rpc.foobar.service.consul", "foobar", "rpc")

	parsedAs(t, "foobar.service.site.consul", "foobar", "")
	parsedAs(t, "http.foobar.service.site.consul", "foobar", "http")
	parsedAs(t, "rpc.foobar.service.site.consul", "foobar", "rpc")
}

# Consul Port Redirector

A basic HTTP server written in Go, which when a request is received with a
HTTP Host header ending in `.service.consul`, queries Consul for the service
port(s) which are matched by that Consul DNS query, and either redirects to
or displays a list of those service ports.

This is intended to run as a Nomad system job, running on every Nomad Client
on port 80. When deployed as such, then Nomad services can be accessed in
a web browser via their Consul Service names, without remembering their
port numbers and while also allowing easy access to individual instances of
a service.

## Setup

First, ensure that Nomad is properly configured to utilize Consul,
and that every Nomad client has a running Consul agent.

Apply `port-redirector.nomad.hcl` to an existing Nomad cluster:
```bash
$ nomad job run port-redirector.nomad.hcl
```

Note that because the job binds on port 80, Nomad must be running
as root on all clients. You can adjust which nodes the job is
deployed on using a constraint.

## Usage

To see all ports registered in Consul for the `nomad` service:

* http://nomad.service.consul
* http://nomad.service.[datacenter].consul


To see all ports named `http` in Consul for the `nomad` service:

* http://http.nomad.service.consul
* http://http.nomad.service.[datacenter].consul

If only one instance matches in Consul, you will be redirected to that port.
If multiple match, you will receive a basic list of linked host:ports.

job "port-redirector" {
    datacenters = ["dc1"]

    type = "system"

    group "default" {
        count = 1
        network {
            port "http" {
                static = 80
            }
        }

        task "default" {
            driver = "docker"

            config {
                image = "jwoglom/consul-port-redirector"
                ports = ["http"]
            }

            env {
                CONSUL_HTTP_ADDR = "http://${attr.unique.hostname}:8500"
            }

            resources {
                cpu = 50
                memory = 64
            }
        }
    }
}

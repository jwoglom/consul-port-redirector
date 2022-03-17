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

                command = "main"
                
                # Add arguments here, if necessary
                args = [

                ]
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

    service {
        name = "port-redirector"
        port = "http"

        check {
            type = "http"

            path = "/healthy"
            interval = "10s"
            timeout = "2s"

            check_restart {
                limit = 5
                grace = "10s"
            }
        }
    }
}

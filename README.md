# prom-logmonitor

Monitor files for configured patterns and report counters for Prometheus.
Support monitoring SystemD Journal.

## Building and running

### Dependency

* github.com/prometheus/client_golang/
* github.com/prometheus/common/version
* github.com/sirupsen/logrus
* github.com/hpcloud/tail
* gopkg.in/yaml.v2
* github.com/Merovius/systemd

For SystemD Journal support also require:
* CGO
* libsystemd


### Local Build & Run

Without systemd:

    go build

with systemd:

	go -tags 'sdjournal'

or use Makefile (propably need edit)

    make (build | build_pi)

Run:

    ./logmonitor


See logmonitor.yml for sample config file

### Options

* `-config.file string` Path to configuration file. (default `eventdb.yml`)
* `-log.format value` Set the log target and format. Example:
  `logger:syslog?appname=bob&local=7` or `logger:stdout?json=true`
  (default `logger:stderr`)
* `-log.level value` Only log messages with the given severity or above. Valid
  levels: [debug, info, warn, error, fatal] (default `info`)
* `-version` Print version information.
* `-web.listen-address string` Address to listen on for web interface and
  telemetry. (default `:9701`)


# License
Copyright (c) 2017, Karol Będkowski.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

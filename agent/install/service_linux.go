package install

import (
	"github.com/aliyun/aliyun_assist_client/thirdparty/service"

	"github.com/aliyun/aliyun_assist_client/agent/util"
)

const (
	systemdScript = `[Unit]
Description={{.Description}}
ConditionFileIsExecutable={{.Path|cmdEscape}}
{{range $i, $dep := .Dependencies}} 
{{$dep}} {{end}}

[Service]
StandardOutput=journal+console
StandardError=journal+console
StartLimitInterval=3600
StartLimitBurst=10
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
{{if .ChRoot}}RootDirectory={{.ChRoot|cmd}}{{end}}
{{if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory|cmdEscape}}{{end}}
{{if .UserName}}User={{.UserName}}{{end}}
{{if .ReloadSignal}}ExecReload=/bin/kill -{{.ReloadSignal}} "$MAINPID"{{end}}
{{if .PIDFile}}PIDFile={{.PIDFile|cmd}}{{end}}
{{if and .LogOutput .HasOutputFileSupport -}}
StandardOutput=file:/var/log/{{.Name}}.out
StandardError=file:/var/log/{{.Name}}.err
{{- end}}
{{if gt .LimitNOFILE -1 }}LimitNOFILE={{.LimitNOFILE}}{{end}}
{{if .Restart}}Restart={{.Restart}}{{end}}
{{if .SuccessExitStatus}}SuccessExitStatus={{.SuccessExitStatus}}{{end}}
RestartSec=120
EnvironmentFile=-/etc/sysconfig/{{.Name}}
KillMode=process
[Install]
WantedBy=multi-user.target
`

	// NOTE: sysvinit script below is copied from https://github.com/kardianos/service/master/service_sysv_linux.go
	// with modification:
	// * In chkconfig configuration [default_runlevels start_priority stop_priority],
	//   runlevels to be started by default is modified which should be consistent
	//   with LSB init configuration below. And two priorities is also modified
	//   to be consistent with installing code in library.
	// * "Required-Start" part of LSB init configuration is modified according to
	//   https://refspecs.linuxbase.org/LSB_3.1.1/LSB-Core-generic/LSB-Core-generic/facilname.html
	//   for service dependency ordering, like what systemd provides.
	// See https://linux.die.net/man/8/chkconfig for more details
	sysvScript = `#!/bin/sh
# For RedHat and cousins:
# chkconfig: 2345 50 02
# description: {{.Description}}
# processname: {{.Path}}

### BEGIN INIT INFO
# Provides:          {{.Path}}
# Required-Start:    $local_fs $network $named $remote_fs
# Required-Stop:
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: {{.DisplayName}}
# Description:       {{.Description}}
### END INIT INFO

cmd="{{.Path}}{{range .Arguments}} {{.|cmd}}{{end}}"

name=$(basename $(readlink -f $0))
pid_file="/var/run/$name.pid"
stdout_log="/var/log/$name.log"
stderr_log="/var/log/$name.err"

[ -e /etc/sysconfig/$name ] && . /etc/sysconfig/$name

get_pid() {
	cat "$pid_file"
}

is_running() {
	[ -f "$pid_file" ] && ps $(get_pid) > /dev/null 2>&1
}

case "$1" in
	start)
		if is_running; then
			echo "Already started"
		else
			echo "Starting $name"
			{{if .WorkingDirectory}}cd '{{.WorkingDirectory}}'{{end}}
			$cmd >> "$stdout_log" 2>> "$stderr_log" &
			echo $! > "$pid_file"
			if ! is_running; then
				echo "Unable to start, see $stdout_log and $stderr_log"
				exit 1
			fi
		fi
	;;
	stop)
		if is_running; then
			echo -n "Stopping $name.."
			kill $(get_pid)
			for i in $(seq 1 10)
			do
				if ! is_running; then
					break
				fi
				echo -n "."
				sleep 1
			done
			echo
			if is_running; then
				echo "Not stopped; may still be shutting down or shutdown may have failed"
				exit 1
			else
				echo "Stopped"
				if [ -f "$pid_file" ]; then
					rm "$pid_file"
				fi
			fi
		else
			echo "Not running"
		fi
	;;
	restart)
		$0 stop
		if is_running; then
			echo "Unable to stop, will not attempt to start"
			exit 1
		fi
		$0 start
	;;
	status)
		if is_running; then
			echo "Running"
		else
			echo "Stopped"
			exit 1
		fi
	;;
	*)
	echo "Usage: $0 {start|stop|restart|status}"
	exit 1
	;;
esac
exit 0
`
)

func ServiceConfig() *service.Config {
	ServiceName := ""
	depends := []string{}
	option := make(service.KeyValue)
	if util.IsSystemdLinux() {
		ServiceName = "aliyun"
		// Official doc https://www.freedesktop.org/wiki/Software/systemd/NetworkTarget/
		// suggests both After= and Wants= configuration to delay a service after
		// network is up. Need validation on ALL distros and releases.
		depends = append(depends, "After=network-online.target")
		depends = append(depends, "Wants=network-online.target")
		option["SystemdScript"] = systemdScript
		option["Restart"] = "on-failure"

		// REMEMBER: Explicit disable LogOutput option of kardianos/service, and
		// use StandardOutput/StandardError settings manually written above.
		option["LogOutput"] = false
	} else {
		ServiceName = "aliyun-service"
		option["SysvScript"] = sysvScript
	}

	return &service.Config{
		// 服务显示名称
		Name: ServiceName,
		// 服务名称
		DisplayName: "Aliyun Assist Service",
		// 服务描述
		Description: "Aliyun Assist",
		// TODO: Use symbolic link for shorter path needs more adaption
		// Executable: "/usr/sbin/aliyun-service",
		Dependencies: depends, //云助手依赖网络
		Option:       option,
	}
}

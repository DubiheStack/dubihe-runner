#!/bin/bash

# Dubihe Runner 守护进程启动脚本

# 默认配置
CONFIG_FILE="config.yaml"
LOG_FILE="/var/log/dubihe-runner.log"
PID_FILE="/var/run/dubihe-runner.pid"

# 日志函数
log_info() {
    echo "$(date '+%Y-%m-%d %H:%M:%S.%3N') INFO  [daemon] $1" | tee -a "$LOG_FILE"
}

log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S.%3N') ERROR [daemon] $1" | tee -a "$LOG_FILE"
}

log_debug() {
    echo "$(date '+%Y-%m-%d %H:%M:%S.%3N') DEBUG [daemon] $1" | tee -a "$LOG_FILE"
}

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo "选项:"
    echo "  start     启动守护进程"
    echo "  stop      停止守护进程"
    echo "  restart   重启守护进程"
    echo "  status    查看运行状态"
    echo "  help      显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  CONFIG_FILE  配置文件路径 (默认: config.yaml)"
    echo "  LOG_FILE     日志文件路径 (默认: /var/log/dubihe-runner.log)"
    echo "  PID_FILE     PID文件路径 (默认: /var/run/dubihe-runner.pid)"
}

# 检查是否已经运行
is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0
        else
            # PID文件存在但进程不存在，删除PID文件
            rm -f "$PID_FILE"
            return 1
        fi
    else
        return 1
    fi
}

# 启动守护进程
start() {
    if is_running; then
        log_info "Dubihe Runner 守护进程已经在运行 (PID: $(cat $PID_FILE))"
        return 1
    fi

    log_info "正在启动 Dubihe Runner 守护进程..."

    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")"
    
    # 确保 PID 目录存在
    mkdir -p "$(dirname "$PID_FILE")"

    # 启动进程，通过--log-file参数指定日志文件
    nohup ./dubihe daemon --config="$CONFIG_FILE" --log-file="$LOG_FILE" >> "$LOG_FILE" 2>&1 &
    local pid=$!
    
    # 保存 PID
    echo $pid > "$PID_FILE"
    
    # 等待一段时间确认进程启动
    sleep 2
    
    if ps -p $pid > /dev/null 2>&1; then
        log_info "Dubihe Runner 守护进程启动成功 (PID: $pid)"
        return 0
    else
        log_error "Dubihe Runner 守护进程启动失败"
        rm -f "$PID_FILE"
        return 1
    fi
}

# 停止守护进程
stop() {
    if ! is_running; then
        log_info "Dubihe Runner 守护进程未在运行"
        return 1
    fi

    local pid=$(cat "$PID_FILE")
    log_info "正在停止 Dubihe Runner 守护进程 (PID: $pid)..."

    # 发送终止信号
    kill -TERM $pid
    
    # 等待进程结束
    local count=0
    while ps -p $pid > /dev/null 2>&1; do
        sleep 1
        count=$((count + 1))
        if [ $count -gt 10 ]; then
            log_error "进程未正常退出，强制终止..."
            kill -KILL $pid
            break
        fi
    done
    
    # 删除 PID 文件
    rm -f "$PID_FILE"
    log_info "Dubihe Runner 守护进程已停止"
}

# 查看状态
status() {
    if is_running; then
        local pid=$(cat "$PID_FILE")
        log_info "Dubihe Runner 守护进程正在运行 (PID: $pid)"
        
        # 显示进程信息
        ps -p $pid -o pid,ppid,cmd,etime
        
        # 显示日志最后几行
        echo ""
        log_info "最近日志:"
        if [ -f "$LOG_FILE" ]; then
            tail -n 10 "$LOG_FILE"
        else
            log_error "日志文件不存在: $LOG_FILE"
        fi
    else
        log_info "Dubihe Runner 守护进程未在运行"
    fi
}

# 重启守护进程
restart() {
    stop
    sleep 2
    start
}

# 主逻辑
case "${1:-}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "未知命令: $1"
        show_help
        exit 1
        ;;
esac
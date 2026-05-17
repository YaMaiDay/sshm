package dbmonitor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

type Runtime struct {
	Container string
}

type Detail struct {
	Configured      bool
	Engine          string
	Host            string
	Port            string
	User            string
	Database        string
	Version         string
	Uptime          string
	Connections     string
	MaxConnections  string
	ActiveConns     string
	IdleConns       string
	DatabaseCount   string
	CacheHit        string
	LockWaits       string
	LongTx          string
	Deadlocks       string
	Questions       string
	SlowQueries     string
	SizeBytes       uint64
	TotalBytes      uint64
	DBTotalBytes    uint64
	DatabaseTop     []TableSize
	DataBytes       uint64
	IndexBytes      uint64
	IndexTotalBytes uint64
	TableCount      string
	TableTop        []TableSize
	MemoryUsed      string
	MemoryPeak      string
	OpsPerSec       string
	Clients         string
	Keyspace        string
	Raw             map[string]string
}

type TableSize struct {
	Name string
	Size uint64
}

func EngineChoices() []string {
	return []string{"MySQL", "MariaDB", "PostgreSQL", "Redis", "MongoDB"}
}

func NormalizeEngine(engine string) string {
	engine = strings.ToLower(strings.TrimSpace(engine))
	switch {
	case strings.Contains(engine, "mariadb"):
		return "MariaDB"
	case strings.Contains(engine, "post"), strings.Contains(engine, "pgsql"):
		return "PostgreSQL"
	case strings.Contains(engine, "redis"):
		return "Redis"
	case strings.Contains(engine, "mongo"):
		return "MongoDB"
	case strings.Contains(engine, "mysql"):
		return "MySQL"
	default:
		for _, choice := range EngineChoices() {
			if strings.EqualFold(choice, engine) {
				return choice
			}
		}
	}
	return ""
}

func DefaultPort(engine string) string {
	switch NormalizeEngine(engine) {
	case "MySQL", "MariaDB":
		return "3306"
	case "PostgreSQL":
		return "5432"
	case "Redis":
		return "6379"
	case "MongoDB":
		return "27017"
	default:
		return ""
	}
}

func DefaultUser(engine string) string {
	switch NormalizeEngine(engine) {
	case "PostgreSQL":
		return "postgres"
	case "Redis", "MongoDB":
		return ""
	default:
		return "root"
	}
}

func DefaultName(engine string) string {
	if NormalizeEngine(engine) == "PostgreSQL" {
		return "postgres"
	}
	return ""
}

func MetricScript(item config.ManagedResource) string {
	return MetricScriptForRuntime(item, Runtime{})
}

func MetricScriptForRuntime(item config.ManagedResource, runtime Runtime) string {
	engine := strings.ToLower(strings.TrimSpace(firstNonEmpty(item.DBEngine, "mysql")))
	host := firstNonEmpty(item.DBHost, "127.0.0.1")
	port := item.DBPort
	if port == "" {
		port = DefaultPort(item.DBEngine)
	}
	switch {
	case strings.Contains(engine, "redis"):
		return redisMetricScript(host, port, item.DBPassword)
	case strings.Contains(engine, "post"):
		return postgresMetricScript(host, firstNonEmpty(port, "5432"), item.DBUser, item.DBPassword, firstNonEmpty(item.DBName, "postgres"), runtime.Container)
	case strings.Contains(engine, "mongo"):
		return mongoMetricScript(host, firstNonEmpty(port, "27017"), item.DBUser, item.DBPassword, item.DBName)
	default:
		return mysqlMetricScript(host, firstNonEmpty(port, "3306"), item.DBUser, item.DBPassword)
	}
}

func mysqlMetricScript(host string, port string, user string, password string) string {
	query := strings.Join([]string{
		"SELECT CONCAT('__SSHM_DB__\\tVERSION\\t', VERSION());",
		"SHOW GLOBAL STATUS WHERE Variable_name IN ('Uptime','Threads_connected','Questions','Slow_queries');",
		"SHOW GLOBAL VARIABLES WHERE Variable_name IN ('max_connections');",
		"SELECT CONCAT('__SSHM_DB__\\tSIZE_BYTES\\t', COALESCE(SUM(data_length+index_length),0)) FROM information_schema.tables;",
		"SELECT CONCAT('__SSHM_DB__\\tDATA_BYTES\\t', COALESCE(SUM(data_length),0)) FROM information_schema.tables;",
		"SELECT CONCAT('__SSHM_DB__\\tINDEX_BYTES\\t', COALESCE(SUM(index_length),0)) FROM information_schema.tables;",
	}, " ")
	return fmt.Sprintf(`DB_CLIENT=""
if command -v mysql >/dev/null 2>&1; then DB_CLIENT="$(command -v mysql)"; elif command -v mariadb >/dev/null 2>&1; then DB_CLIENT="$(command -v mariadb)"; fi
if [ -z "$DB_CLIENT" ]; then echo "__SSHM_DB_ERROR__ mysql/mariadb客户端不可用"; exit 0; fi
MYSQL_PWD=%s "$DB_CLIENT" -N -B -h %s -P %s -u %s -e %s 2>&1 | awk 'BEGIN{FS="\t"} /^__SSHM_DB__/ {print; next} NF>=2 {print "__SSHM_DB__\t"$1"\t"$2}'`,
		remotescript.Quote(password), remotescript.Quote(host), remotescript.Quote(port), remotescript.Quote(firstNonEmpty(user, "root")), remotescript.Quote(query))
}

func redisMetricScript(host string, port string, password string) string {
	auth := ""
	if strings.TrimSpace(password) != "" {
		auth = " -a " + remotescript.Quote(password)
	}
	return fmt.Sprintf(`if ! command -v redis-cli >/dev/null 2>&1; then echo "__SSHM_DB_ERROR__ redis-cli不可用"; exit 0; fi
redis-cli -h %s -p %s%s INFO 2>&1 | awk -F: '
$1=="redis_version"{print "__SSHM_DB__\tVERSION\t"$2}
$1=="uptime_in_seconds"{print "__SSHM_DB__\tUptime\t"$2}
$1=="connected_clients"{print "__SSHM_DB__\tThreads_connected\t"$2}
$1=="used_memory_human"{print "__SSHM_DB__\tMEMORY_USED\t"$2}
$1=="used_memory_peak_human"{print "__SSHM_DB__\tMEMORY_PEAK\t"$2}
$1=="instantaneous_ops_per_sec"{print "__SSHM_DB__\tOPS_PER_SEC\t"$2}
$1 ~ /^db[0-9]+$/ {print "__SSHM_DB__\tKEYSPACE\t"$1":"$2}
'`, remotescript.Quote(host), remotescript.Quote(firstNonEmpty(port, "6379")), auth)
}

func postgresMetricScript(host string, port string, user string, password string, database string, container string) string {
	query := strings.Join([]string{
		"SELECT '__SSHM_DB__'||chr(9)||'VERSION'||chr(9)||version();",
		"SELECT '__SSHM_DB__'||chr(9)||'Uptime'||chr(9)||EXTRACT(EPOCH FROM now()-pg_postmaster_start_time())::bigint;",
		"SELECT '__SSHM_DB__'||chr(9)||'SIZE_BYTES'||chr(9)||pg_database_size(current_database());",
		"SELECT '__SSHM_DB__'||chr(9)||'DATA_DIR'||chr(9)||current_setting('data_directory');",
		"SELECT '__SSHM_DB__'||chr(9)||'Threads_connected'||chr(9)||count(*) FROM pg_stat_activity;",
		"SELECT '__SSHM_DB__'||chr(9)||'Max_connections'||chr(9)||setting FROM pg_settings WHERE name='max_connections';",
		"SELECT '__SSHM_DB__'||chr(9)||'DATABASE_COUNT'||chr(9)||count(*) FROM pg_database WHERE datallowconn;",
		"SELECT '__SSHM_DB__'||chr(9)||'DATABASE_TOTAL_BYTES'||chr(9)||coalesce(sum(pg_database_size(datname)),0) FROM pg_database WHERE datallowconn;",
		"SELECT '__SSHM_DB__'||chr(9)||'DATABASE_TOP'||chr(9)||datname||'='||pg_database_size(datname) FROM pg_database WHERE datallowconn ORDER BY pg_database_size(datname) DESC LIMIT 10;",
		"SELECT '__SSHM_DB__'||chr(9)||'INDEX_TOTAL_BYTES'||chr(9)||coalesce(sum(pg_relation_size(indexrelid)),0) FROM pg_index;",
		"SELECT '__SSHM_DB__'||chr(9)||'TABLE_COUNT'||chr(9)||count(*) FROM pg_stat_user_tables;",
		"SELECT '__SSHM_DB__'||chr(9)||'CACHE_HIT'||chr(9)||CASE WHEN blks_hit+blks_read=0 THEN 0 ELSE round(100.0*blks_hit/(blks_hit+blks_read),2) END FROM pg_stat_database WHERE datname=current_database();",
		"SELECT '__SSHM_DB__'||chr(9)||'CONN_ACTIVE'||chr(9)||count(*) FROM pg_stat_activity WHERE state='active';",
		"SELECT '__SSHM_DB__'||chr(9)||'CONN_IDLE'||chr(9)||count(*) FROM pg_stat_activity WHERE state='idle';",
		"SELECT '__SSHM_DB__'||chr(9)||'LOCK_WAITS'||chr(9)||count(*) FROM pg_stat_activity WHERE wait_event_type='Lock';",
		"SELECT '__SSHM_DB__'||chr(9)||'LONG_TX'||chr(9)||count(*) FROM pg_stat_activity WHERE xact_start IS NOT NULL AND state<>'idle' AND now()-xact_start > interval '5 minutes';",
		"SELECT '__SSHM_DB__'||chr(9)||'DEADLOCKS'||chr(9)||deadlocks FROM pg_stat_database WHERE datname=current_database();",
		"SELECT '__SSHM_DB__'||chr(9)||'TABLE_TOP'||chr(9)||schemaname||'.'||relname||'='||pg_total_relation_size(relid) FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC LIMIT 10;",
	}, " ")
	return fmt.Sprintf(`DB_CONTAINER=%s
if [ -n "$DB_CONTAINER" ] && command -v docker >/dev/null 2>&1; then
  run_docker() { docker "$@" 2>&1; }
  run_docker_sudo() { sudo -n docker "$@" 2>&1; }
  out=$(run_docker exec -e %s "$DB_CONTAINER" psql -h 127.0.0.1 -p 5432 -U %s -d %s -Atc %s)
  code=$?
  if [ "$code" -ne 0 ]; then
    out=$(run_docker_sudo exec -e %s "$DB_CONTAINER" psql -h 127.0.0.1 -p 5432 -U %s -d %s -Atc %s)
    code=$?
  fi
  if [ "$code" -eq 0 ]; then
    printf '%%s\n' "$out"
    data_dir=$(printf '%%s\n' "$out" | awk -F '	' '$1=="__SSHM_DB__" && $2=="DATA_DIR"{print $3; exit}')
    if [ -z "$data_dir" ]; then data_dir="${PGDATA:-/var/lib/postgresql/data}"; fi
    fs=$(run_docker exec "$DB_CONTAINER" df -PB1 "$data_dir" 2>/dev/null)
    fs_code=$?
    if [ "$fs_code" -ne 0 ]; then
      fs=$(run_docker_sudo exec "$DB_CONTAINER" df -PB1 "$data_dir" 2>/dev/null)
      fs_code=$?
    fi
    if [ "$fs_code" -eq 0 ]; then
      printf '%%s\n' "$fs" | awk 'NR==2 {print "__SSHM_DB__\tTOTAL_BYTES\t"$2}'
    fi
    exit 0
  fi
fi
if command -v psql >/dev/null 2>&1; then
  out=$(PGPASSWORD=%s psql -h %s -p %s -U %s -d %s -Atc %s 2>&1)
  code=$?
  printf '%%s\n' "$out"
  data_dir=$(printf '%%s\n' "$out" | awk -F '	' '$1=="__SSHM_DB__" && $2=="DATA_DIR"{print $3; exit}')
  if [ "$code" -eq 0 ] && [ -n "$data_dir" ] && [ -d "$data_dir" ]; then
    df -PB1 "$data_dir" 2>/dev/null | awk 'NR==2 {print "__SSHM_DB__\tTOTAL_BYTES\t"$2}'
  fi
  exit 0
fi
echo "__SSHM_DB_ERROR__ psql客户端不可用"`,
		remotescript.Quote(container),
		remotescript.Quote("PGPASSWORD="+password), remotescript.Quote(firstNonEmpty(user, "postgres")), remotescript.Quote(database), remotescript.Quote(query),
		remotescript.Quote("PGPASSWORD="+password), remotescript.Quote(firstNonEmpty(user, "postgres")), remotescript.Quote(database), remotescript.Quote(query),
		remotescript.Quote(password), remotescript.Quote(host), remotescript.Quote(port), remotescript.Quote(firstNonEmpty(user, "postgres")), remotescript.Quote(database), remotescript.Quote(query))
}

func mongoMetricScript(host string, port string, user string, password string, database string) string {
	database = firstNonEmpty(database, "admin")
	auth := ""
	if strings.TrimSpace(user) != "" {
		auth = " -u " + remotescript.Quote(user)
	}
	if strings.TrimSpace(password) != "" {
		auth += " -p " + remotescript.Quote(password)
	}
	if strings.TrimSpace(user) != "" {
		auth += " --authenticationDatabase " + remotescript.Quote(database)
	}
	eval := `var s=db.serverStatus(); var st=db.stats(); print('__SSHM_DB__\tVERSION\t'+s.version); print('__SSHM_DB__\tUptime\t'+s.uptime); print('__SSHM_DB__\tThreads_connected\t'+(s.connections?s.connections.current:'')); print('__SSHM_DB__\tMax_connections\t'+(s.connections?s.connections.available+s.connections.current:'')); print('__SSHM_DB__\tSIZE_BYTES\t'+(st.storageSize||0)); print('__SSHM_DB__\tDATA_BYTES\t'+(st.dataSize||0)); print('__SSHM_DB__\tINDEX_BYTES\t'+(st.indexSize||0));`
	return fmt.Sprintf(`MONGO_CLIENT=""
if command -v mongosh >/dev/null 2>&1; then MONGO_CLIENT="$(command -v mongosh)"; elif command -v mongo >/dev/null 2>&1; then MONGO_CLIENT="$(command -v mongo)"; fi
if [ -z "$MONGO_CLIENT" ]; then echo "__SSHM_DB_ERROR__ mongosh/mongo客户端不可用"; exit 0; fi
"$MONGO_CLIENT" --quiet --host %s --port %s%s %s --eval %s 2>&1`,
		remotescript.Quote(host), remotescript.Quote(port), auth, remotescript.Quote(database), remotescript.Quote(eval))
}

func Parse(output string) (Detail, string) {
	detail := Detail{Configured: true, Raw: map[string]string{}}
	errText := ""
	firstOutput := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "__SSHM_DB_ERROR__") {
			errText = strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_DB_ERROR__"))
			continue
		}
		if !strings.HasPrefix(line, "__SSHM_DB__\t") {
			if firstOutput == "" && !isIgnorableOutputLine(line) {
				firstOutput = line
			}
			if errText == "" && strings.Contains(strings.ToLower(line), "error") {
				errText = line
			}
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		key := strings.TrimSpace(parts[1])
		value := strings.TrimSpace(parts[2])
		detail.Raw[key] = value
		switch strings.ToLower(key) {
		case "version":
			detail.Version = value
		case "uptime":
			detail.Uptime = formatSeconds(value)
		case "threads_connected":
			detail.Connections = value
		case "max_connections":
			detail.MaxConnections = value
		case "conn_active":
			detail.ActiveConns = value
		case "conn_idle":
			detail.IdleConns = value
		case "database_count":
			detail.DatabaseCount = value
		case "cache_hit":
			detail.CacheHit = percentSuffix(value)
		case "lock_waits":
			detail.LockWaits = value
		case "long_tx":
			detail.LongTx = value
		case "deadlocks":
			detail.Deadlocks = value
		case "questions":
			detail.Questions = value
		case "slow_queries":
			detail.SlowQueries = value
		case "size_bytes":
			detail.SizeBytes = parseUint64(value)
		case "total_bytes":
			detail.TotalBytes = parseUint64(value)
		case "database_total_bytes":
			detail.DBTotalBytes = parseUint64(value)
		case "database_top":
			if database, ok := parseTableTop(value); ok {
				detail.DatabaseTop = append(detail.DatabaseTop, database)
			}
		case "data_bytes":
			detail.DataBytes = parseUint64(value)
		case "index_bytes":
			detail.IndexBytes = parseUint64(value)
		case "index_total_bytes":
			detail.IndexTotalBytes = parseUint64(value)
		case "table_count":
			detail.TableCount = value
		case "table_top":
			if table, ok := parseTableTop(value); ok {
				detail.TableTop = append(detail.TableTop, table)
			}
		case "memory_used":
			detail.MemoryUsed = value
		case "memory_peak":
			detail.MemoryPeak = value
		case "ops_per_sec":
			detail.OpsPerSec = value
		case "keyspace":
			detail.Keyspace = appendUniqueCSV(detail.Keyspace, value)
		}
	}
	if detail.Version == "" && errText == "" {
		if firstOutput != "" {
			errText = "没有获取到数据库指标：" + truncateError(firstOutput)
		} else {
			errText = "没有获取到数据库指标"
		}
	}
	return detail, errText
}

func MissingCredentialHint(item config.ManagedResource, errText string) bool {
	errText = strings.TrimSpace(errText)
	if errText == "" || strings.TrimSpace(item.DBPassword) != "" {
		return false
	}
	switch NormalizeEngine(item.DBEngine) {
	case "MySQL", "MariaDB", "PostgreSQL":
	default:
		return false
	}
	lower := strings.ToLower(errText)
	return strings.HasPrefix(errText, "没有获取到数据库指标") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "password")
}

func parseTableTop(value string) (TableSize, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return TableSize{}, false
	}
	name, sizeText, ok := strings.Cut(value, "=")
	if !ok {
		return TableSize{Name: value}, true
	}
	size := parseUint64(sizeText)
	return TableSize{Name: strings.TrimSpace(name), Size: size}, true
}

func isIgnorableOutputLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return strings.Contains(line, "Deprecated program name")
}

func truncateError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120]) + "..."
}

func parseUint64(value string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func formatSeconds(value string) string {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n < 0 {
		return value
	}
	days := n / 86400
	hours := (n % 86400) / 3600
	minutes := (n % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func percentSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, "%") {
		return value
	}
	return value + "%"
}

func appendUniqueCSV(current string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return current
	}
	values := splitCSV(current)
	for _, existing := range values {
		if existing == value {
			return current
		}
	}
	values = append(values, value)
	return strings.Join(values, ", ")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

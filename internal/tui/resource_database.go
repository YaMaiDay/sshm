package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
)

func deriveDatabaseDetails(services []serviceDetail, containers []containerDetail, ports []portDetail) ([]databaseDetail, string) {
	items := []databaseDetail{}
	for _, item := range containers {
		engine := databaseEngineFromContainer(item)
		if engine == "" {
			continue
		}
		db := databaseDetail{
			Name:      firstNonEmpty(strings.TrimSpace(item.Name), engine),
			Engine:    engine,
			Source:    "Docker",
			Status:    databaseStatusFromContainer(item),
			RawStatus: item.Status,
			Endpoint:  firstDatabaseEndpointFromPorts(item.Ports, engine),
			Container: item.Name,
			Image:     item.Image,
		}
		items = append(items, db)
	}
	for _, item := range services {
		engine := databaseEngineFromService(item)
		if engine == "" {
			continue
		}
		db := databaseDetail{
			Name:        firstNonEmpty(strings.TrimSuffix(strings.TrimSpace(item.Unit), ".service"), engine),
			Engine:      engine,
			Source:      "systemd",
			Status:      databaseStatusFromService(item),
			RawStatus:   serviceRawState(item),
			ServiceUnit: item.Unit,
			Process:     serviceProgramName(item),
			PID:         servicePIDText(item),
		}
		items = mergeDatabaseDetail(items, db)
	}
	for _, item := range ports {
		engine := databaseEngineFromPort(item)
		if engine == "" {
			continue
		}
		db := databaseDetail{
			Name:        databaseNameFromPort(item, engine),
			Engine:      engine,
			Source:      databasePortSource(item),
			Status:      databaseStatusFromPort(item),
			RawStatus:   emptyDash(item.State),
			Endpoint:    portListenText(item),
			ServiceUnit: item.ServiceUnit,
			Container:   item.Container,
			Process:     item.Process,
			PID:         item.PID,
			Protocol:    item.Protocol,
			Port:        item.Port,
		}
		items = mergeDatabaseDetail(items, db)
	}
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := databaseStatusRank(items[i]), databaseStatusRank(items[j])
		if ri != rj {
			return ri < rj
		}
		if items[i].Engine != items[j].Engine {
			return items[i].Engine < items[j].Engine
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, ""
}

func mergeDatabaseDetail(items []databaseDetail, next databaseDetail) []databaseDetail {
	for i := range items {
		if !sameDatabaseResource(items[i], next) {
			continue
		}
		items[i] = combineDatabaseDetail(items[i], next)
		return items
	}
	return append(items, next)
}

func sameDatabaseResource(a databaseDetail, b databaseDetail) bool {
	if a.Engine != b.Engine {
		return false
	}
	if nonEmptyEqual(a.Container, b.Container) || nonEmptyEqual(a.ServiceUnit, b.ServiceUnit) {
		return true
	}
	if nonEmptyEqual(a.Process, b.Process) && nonEmptyEqual(a.PID, b.PID) {
		return true
	}
	if a.Port != "" && b.Port != "" && a.Port == b.Port && strings.EqualFold(a.Protocol, b.Protocol) {
		return true
	}
	if databaseDefaultPort(a.Engine) != "" && (a.Port == databaseDefaultPort(a.Engine) || b.Port == databaseDefaultPort(a.Engine)) {
		return a.Source != "Docker" && b.Source != "Docker"
	}
	return false
}

func combineDatabaseDetail(a databaseDetail, b databaseDetail) databaseDetail {
	a.Name = firstNonEmpty(a.Name, b.Name)
	a.Source = combineDatabaseSource(a.Source, b.Source)
	a.Status = databaseWorseStatus(a.Status, b.Status)
	a.RawStatus = firstNonEmpty(a.RawStatus, b.RawStatus)
	a.Endpoint = firstNonEmpty(a.Endpoint, b.Endpoint)
	a.ServiceUnit = firstNonEmpty(a.ServiceUnit, b.ServiceUnit)
	a.Container = firstNonEmpty(a.Container, b.Container)
	a.Image = firstNonEmpty(a.Image, b.Image)
	a.Process = firstNonEmpty(a.Process, b.Process)
	a.PID = firstNonEmpty(a.PID, b.PID)
	a.Protocol = firstNonEmpty(a.Protocol, b.Protocol)
	a.Port = firstNonEmpty(a.Port, b.Port)
	return a
}

func combineDatabaseSource(a string, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" || strings.Contains(a, b) {
		return a
	}
	if strings.Contains(b, a) {
		return b
	}
	return a + "+" + b
}

func databaseWorseStatus(a string, b string) string {
	if databaseStatusRank(databaseDetail{Status: b}) < databaseStatusRank(databaseDetail{Status: a}) {
		return b
	}
	return a
}

func databaseEngineFromContainer(item containerDetail) string {
	text := strings.ToLower(strings.Join([]string{item.Name, item.Image}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromService(item serviceDetail) string {
	text := strings.ToLower(strings.Join([]string{item.Unit, item.Description, item.ExecStart, serviceProgramPath(item)}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromPort(item portDetail) string {
	if engine := databaseEngineFromDefaultPort(item.Port); engine != "" {
		return engine
	}
	text := strings.ToLower(strings.Join([]string{item.Process, item.ServiceUnit, item.Container}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromText(text string) string {
	switch {
	case containsDatabaseToken(text, "mariadb"), containsDatabaseToken(text, "mariadbd"):
		return "MariaDB"
	case containsDatabaseToken(text, "mysql"), containsDatabaseToken(text, "mysqld"):
		return "MySQL"
	case containsDatabaseToken(text, "postgres"), containsDatabaseToken(text, "postgresql"):
		return "PostgreSQL"
	case containsDatabaseToken(text, "redis"), containsDatabaseToken(text, "redis-server"):
		return "Redis"
	case containsDatabaseToken(text, "mongodb"), containsDatabaseToken(text, "mongo"), containsDatabaseToken(text, "mongod"):
		return "MongoDB"
	default:
		return ""
	}
}

func containsDatabaseToken(text string, token string) bool {
	text = strings.ToLower(text)
	token = strings.ToLower(token)
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if part == token {
			return true
		}
	}
	return strings.Contains(text, "/"+token+":") || strings.Contains(text, ":"+token+":")
}

func databaseEngineFromDefaultPort(port string) string {
	switch strings.TrimSpace(port) {
	case "3306":
		return "MySQL"
	case "5432":
		return "PostgreSQL"
	case "6379":
		return "Redis"
	case "27017", "27018", "27019":
		return "MongoDB"
	default:
		return ""
	}
}

func databaseDefaultPort(engine string) string {
	switch normalizeDatabaseEngine(engine) {
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

func databaseEngineChoices() []string {
	return []string{"MySQL", "MariaDB", "PostgreSQL", "Redis", "MongoDB"}
}

func normalizeDatabaseEngine(engine string) string {
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
		for _, choice := range databaseEngineChoices() {
			if strings.EqualFold(choice, engine) {
				return choice
			}
		}
	}
	return ""
}

func databaseStatusFromContainer(item containerDetail) string {
	switch containerDetailKind(item) {
	case "failed":
		return "problem"
	case "stopped":
		return "stopped"
	case "missing":
		return "missing"
	case "running":
		return "running"
	default:
		return "unknown"
	}
}

func databaseStatusFromService(item serviceDetail) string {
	switch serviceDetailKind(item) {
	case "failed":
		return "problem"
	case "stopped":
		return "stopped"
	case "missing":
		return "missing"
	case "running", "active":
		return "running"
	default:
		return "unknown"
	}
}

func databaseStatusFromPort(item portDetail) string {
	if item.Missing {
		return "missing"
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	if state == "" || state == "LISTEN" || state == "UNCONN" {
		return "running"
	}
	return "unknown"
}

func databaseStatusRank(item databaseDetail) int {
	switch item.Status {
	case "problem", "missing":
		return 0
	case "running":
		return 1
	case "stopped":
		return 2
	default:
		return 3
	}
}

func firstDatabaseEndpointFromPorts(ports string, engine string) string {
	defaultPort := databaseDefaultPort(engine)
	for _, part := range strings.Split(ports, ",") {
		hostPort, _, _, ok := parseDockerPublishedPort(part)
		if ok && (defaultPort == "" || hostPort == defaultPort) {
			return strings.TrimSpace(part)
		}
	}
	return strings.TrimSpace(ports)
}

func databaseNameFromPort(item portDetail, engine string) string {
	return firstNonEmpty(item.Container, item.ServiceUnit, item.Process, engine)
}

func databasePortSource(item portDetail) string {
	if strings.TrimSpace(item.Container) != "" {
		return "Docker+port"
	}
	if strings.TrimSpace(item.ServiceUnit) != "" {
		return "systemd+port"
	}
	return "port"
}

func serviceProgramName(item serviceDetail) string {
	path := serviceProgramPath(item)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}

func nonEmptyEqual(a string, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func databaseDefaultHost(item databaseDetail) string {
	host, _, ok := databasePublishedEndpoint(item)
	if ok {
		return host
	}
	return "127.0.0.1"
}

func databasePublishedEndpoint(item databaseDetail) (string, string, bool) {
	defaultPort := databaseDefaultPort(item.Engine)
	for _, part := range strings.Split(item.Endpoint, ",") {
		hostPort, targetPort, _, ok := parseDockerPublishedPort(part)
		if ok && (defaultPort == "" || targetPort == defaultPort) {
			return "127.0.0.1", hostPort, true
		}
	}
	if strings.TrimSpace(item.Port) != "" {
		return "127.0.0.1", strings.TrimSpace(item.Port), true
	}
	return "", "", false
}

func databaseDefaultPortForDetail(item databaseDetail) string {
	_, port, ok := databasePublishedEndpoint(item)
	if ok {
		return port
	}
	endpoint := strings.TrimSpace(item.Endpoint)
	if endpoint != "" && !strings.Contains(endpoint, "->") {
		port := portFromAddress(endpoint)
		if port != "" {
			return port
		}
	}
	if strings.TrimSpace(item.Port) != "" {
		return strings.TrimSpace(item.Port)
	}
	return databaseDefaultPort(item.Engine)
}

func databaseDefaultUser(engine string) string {
	switch normalizeDatabaseEngine(engine) {
	case "PostgreSQL":
		return "postgres"
	case "Redis":
		return ""
	case "MongoDB":
		return ""
	default:
		return "root"
	}
}

func databaseDefaultName(engine string) string {
	if normalizeDatabaseEngine(engine) == "PostgreSQL" {
		return "postgres"
	}
	return ""
}

func databaseMetricScript(item config.ManagedResource) string {
	return databaseMetricScriptForDetail(item, databaseDetail{})
}

func databaseMetricScriptForDetail(item config.ManagedResource, detail databaseDetail) string {
	engine := strings.ToLower(strings.TrimSpace(firstNonEmpty(item.DBEngine, "mysql")))
	host := firstNonEmpty(item.DBHost, "127.0.0.1")
	port := item.DBPort
	if port == "" {
		port = databaseDefaultPort(item.DBEngine)
	}
	switch {
	case strings.Contains(engine, "redis"):
		return redisMetricScript(host, port, item.DBPassword)
	case strings.Contains(engine, "post"):
		return postgresMetricScript(host, firstNonEmpty(port, "5432"), item.DBUser, item.DBPassword, firstNonEmpty(item.DBName, "postgres"), detail.Container)
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
		shellQuoteLocal(password), shellQuoteLocal(host), shellQuoteLocal(port), shellQuoteLocal(firstNonEmpty(user, "root")), shellQuoteLocal(query))
}

func redisMetricScript(host string, port string, password string) string {
	auth := ""
	if strings.TrimSpace(password) != "" {
		auth = " -a " + shellQuoteLocal(password)
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
'`, shellQuoteLocal(host), shellQuoteLocal(firstNonEmpty(port, "6379")), auth)
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
		shellQuoteLocal(container),
		shellQuoteLocal("PGPASSWORD="+password), shellQuoteLocal(firstNonEmpty(user, "postgres")), shellQuoteLocal(database), shellQuoteLocal(query),
		shellQuoteLocal("PGPASSWORD="+password), shellQuoteLocal(firstNonEmpty(user, "postgres")), shellQuoteLocal(database), shellQuoteLocal(query),
		shellQuoteLocal(password), shellQuoteLocal(host), shellQuoteLocal(port), shellQuoteLocal(firstNonEmpty(user, "postgres")), shellQuoteLocal(database), shellQuoteLocal(query))
}

func mongoMetricScript(host string, port string, user string, password string, database string) string {
	database = firstNonEmpty(database, "admin")
	auth := ""
	if strings.TrimSpace(user) != "" {
		auth = " -u " + shellQuoteLocal(user)
	}
	if strings.TrimSpace(password) != "" {
		auth += " -p " + shellQuoteLocal(password)
	}
	if strings.TrimSpace(user) != "" {
		auth += " --authenticationDatabase " + shellQuoteLocal(database)
	}
	eval := `var s=db.serverStatus(); var st=db.stats(); print('__SSHM_DB__\tVERSION\t'+s.version); print('__SSHM_DB__\tUptime\t'+s.uptime); print('__SSHM_DB__\tThreads_connected\t'+(s.connections?s.connections.current:'')); print('__SSHM_DB__\tMax_connections\t'+(s.connections?s.connections.available+s.connections.current:'')); print('__SSHM_DB__\tSIZE_BYTES\t'+(st.storageSize||0)); print('__SSHM_DB__\tDATA_BYTES\t'+(st.dataSize||0)); print('__SSHM_DB__\tINDEX_BYTES\t'+(st.indexSize||0));`
	return fmt.Sprintf(`MONGO_CLIENT=""
if command -v mongosh >/dev/null 2>&1; then MONGO_CLIENT="$(command -v mongosh)"; elif command -v mongo >/dev/null 2>&1; then MONGO_CLIENT="$(command -v mongo)"; fi
if [ -z "$MONGO_CLIENT" ]; then echo "__SSHM_DB_ERROR__ mongosh/mongo客户端不可用"; exit 0; fi
"$MONGO_CLIENT" --quiet --host %s --port %s%s %s --eval %s 2>&1`,
		shellQuoteLocal(host), shellQuoteLocal(port), auth, shellQuoteLocal(database), shellQuoteLocal(eval))
}

func parseDatabaseExtraDetail(output string) (databaseExtraDetail, string) {
	detail := databaseExtraDetail{Configured: true, Raw: map[string]string{}}
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
			if firstOutput == "" && !isIgnorableDatabaseOutputLine(line) {
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
			detail.Uptime = formatSecondsText(value)
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
			detail.SizeBytes = parseUint64Text(value)
		case "total_bytes":
			detail.TotalBytes = parseUint64Text(value)
		case "database_total_bytes":
			detail.DBTotalBytes = parseUint64Text(value)
		case "database_top":
			if database, ok := parseDatabaseTableTop(value); ok {
				detail.DatabaseTop = append(detail.DatabaseTop, database)
			}
		case "data_bytes":
			detail.DataBytes = parseUint64Text(value)
		case "index_bytes":
			detail.IndexBytes = parseUint64Text(value)
		case "index_total_bytes":
			detail.IndexTotalBytes = parseUint64Text(value)
		case "table_count":
			detail.TableCount = value
		case "table_top":
			if table, ok := parseDatabaseTableTop(value); ok {
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
			errText = "没有获取到数据库指标：" + truncateDatabaseError(firstOutput)
		} else {
			errText = "没有获取到数据库指标"
		}
	}
	return detail, errText
}

func databaseMissingCredentialHint(item config.ManagedResource, errText string) bool {
	errText = strings.TrimSpace(errText)
	if errText == "" || strings.TrimSpace(item.DBPassword) != "" {
		return false
	}
	switch normalizeDatabaseEngine(item.DBEngine) {
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

func parseDatabaseTableTop(value string) (databaseTableSize, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return databaseTableSize{}, false
	}
	name, sizeText, ok := strings.Cut(value, "=")
	if !ok {
		return databaseTableSize{Name: value}, true
	}
	size := parseUint64Text(sizeText)
	return databaseTableSize{Name: strings.TrimSpace(name), Size: size}, true
}

func isIgnorableDatabaseOutputLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return strings.Contains(line, "Deprecated program name")
}

func truncateDatabaseError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120]) + "..."
}

func parseUint64Text(value string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func formatSecondsText(value string) string {
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

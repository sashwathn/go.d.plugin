// SPDX-License-Identifier: GPL-3.0-or-later

package postgres

import (
	"fmt"
	"strings"
)

// http://sqllint.com/

func queryServerVersion() string {
	return "SHOW server_version_num;"
}

//func queryIsSuperUser() string {
//	return "SELECT current_setting('is_superuser') = 'on' AS is_superuser;"
//}

// TODO: this is not correct and we should use pg_stat_activity.
// But we need to check what connections (backend_type) count towards 'max_connections'.
// I think python version query doesn't count it correctly.
// https://github.com/netdata/netdata/blob/1782e2d002bc5203128e5a5d2b801010e2822d2d/collectors/python.d.plugin/postgres/postgres.chart.py#L266
func queryServerCurrentConnectionsNum() string {
	return "SELECT sum(numbackends) FROM pg_stat_database;"
}

func querySettingsMaxConnections() string {
	return "SELECT current_setting('max_connections')::INT - current_setting('superuser_reserved_connections')::INT;"
}

func queryServerUptime() string {
	return `
SELECT
    EXTRACT(epoch 
FROM
    CURRENT_TIMESTAMP - pg_postmaster_start_time());
`
}

func queryTXIDWraparound() string {
	// https://www.crunchydata.com/blog/managing-transaction-id-wraparound-in-postgresql
	return `
    WITH max_age AS ( SELECT
        2000000000 as max_old_xid,
        setting AS autovacuum_freeze_max_age 
    FROM
        pg_catalog.pg_settings 
    WHERE
        name = 'autovacuum_freeze_max_age'), per_database_stats AS ( SELECT
        datname ,
        m.max_old_xid::int ,
        m.autovacuum_freeze_max_age::int ,
        age(d.datfrozenxid) AS oldest_current_xid 
    FROM
        pg_catalog.pg_database d 
    JOIN
        max_age m 
            ON (true) 
    WHERE
        d.datallowconn) SELECT
        max(oldest_current_xid) AS oldest_current_xid ,
        max(ROUND(100*(oldest_current_xid/max_old_xid::float))) AS percent_towards_wraparound ,
        max(ROUND(100*(oldest_current_xid/autovacuum_freeze_max_age::float))) AS percent_towards_emergency_autovacuum 
    FROM
        per_database_stats;
`
}

func queryDatabaseList() string {
	return `
    SELECT
        datname 
    FROM
        pg_database 
    WHERE
        has_database_privilege((SELECT
            CURRENT_USER), datname, 'connect')     
        AND NOT datname ~* '^template\d' 
    ORDER BY
        datname;
`
}

func queryDatabaseStats(dbs []string) string {
	// definition by version: https://pgpedia.info/p/pg_stat_database.html
	// docs: https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-DATABASE-VIEW
	// code: https://github.com/postgres/postgres/blob/366283961ac0ed6d89014444c6090f3fd02fce0a/src/backend/catalog/system_views.sql#L1018

	q := `
    SELECT
        stat.datname,
        numbackends,
        pg_database.datconnlimit,
        xact_commit,
        xact_rollback,
        blks_read,
        blks_hit,
        tup_returned,
        tup_fetched,
        tup_inserted,
        tup_updated,
        tup_deleted,
        conflicts,
        pg_database_size(stat.datname) AS size,
        temp_files,
        temp_bytes,
        deadlocks     
    FROM
        pg_stat_database stat     
    INNER JOIN
        pg_database                                                                                   
            ON pg_database.datname = stat.datname     
    WHERE
        stat.datname SIMILAR TO '%s';
`
	if len(dbs) == 0 {
		q = fmt.Sprintf(q, "%")
	} else {
		q = fmt.Sprintf(q, strings.Join(dbs, "|"))
	}

	return q
}

func queryDatabaseConflicts(dbs []string) string {
	// definition by version: https://pgpedia.info/p/pg_stat_database_conflicts.html
	// docs: https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-DATABASE-CONFLICTS-VIEW
	// code: https://github.com/postgres/postgres/blob/366283961ac0ed6d89014444c6090f3fd02fce0a/src/backend/catalog/system_views.sql#L1058

	q := `
    SELECT
        datname,
        confl_tablespace,
        confl_lock,
        confl_snapshot,
        confl_bufferpin,
        confl_deadlock     
    FROM
        pg_stat_database_conflicts
    WHERE
        datname SIMILAR TO '%s';
`
	if len(dbs) == 0 {
		q = fmt.Sprintf(q, "%")
	} else {
		q = fmt.Sprintf(q, strings.Join(dbs, "|"))
	}

	return q
}

func queryCheckpoints() string {
	// definition by version: https://pgpedia.info/p/pg_stat_bgwriter.html
	// docs: https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-BGWRITER-VIEW
	// code: https://github.com/postgres/postgres/blob/366283961ac0ed6d89014444c6090f3fd02fce0a/src/backend/catalog/system_views.sql#L1104

	return `
    SELECT
        checkpoints_timed,
        checkpoints_req,
        checkpoint_write_time,
        checkpoint_sync_time,
        buffers_checkpoint * current_setting('block_size')::numeric buffers_checkpoint,
        buffers_clean * current_setting('block_size')::numeric buffers_clean,
        maxwritten_clean,
        buffers_backend * current_setting('block_size')::numeric buffers_backend,
        buffers_backend_fsync,
        buffers_alloc * current_setting('block_size')::numeric buffers_alloc 
    FROM
        pg_stat_bgwriter;
`
}

func queryDatabaseLocks(dbs []string) string {
	// definition by version: https://pgpedia.info/p/pg_locks.html
	// docs: https://www.postgresql.org/docs/current/view-pg-locks.html

	q := `
    SELECT
        pg_database.datname,
        mode,
        granted,
        count(mode) AS locks_count                          
    FROM
        pg_locks                          
    INNER JOIN
        pg_database                                                                      
            ON pg_database.oid = pg_locks.database               
    WHERE
        datname SIMILAR TO '%s'          
    GROUP BY
        datname,
        mode,
        granted                          
    ORDER BY
        datname,
        mode;
`
	if len(dbs) == 0 {
		q = fmt.Sprintf(q, "%")
	} else {
		q = fmt.Sprintf(q, strings.Join(dbs, "|"))
	}

	return q
}
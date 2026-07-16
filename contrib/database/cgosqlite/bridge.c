//go:build cgo

#define SQLITE_THREADSAFE 1
#define SQLITE_OMIT_LOAD_EXTENSION 1
#define SQLITE_DQS 0
#define SQLITE_DEFAULT_MEMSTATUS 0
#define SQLITE_USE_URI 1
#define SQLITE_DEFAULT_FOREIGN_KEYS 1
#define SQLITE_MAX_SQL_LENGTH 1048576
#define SQLITE_MAX_LENGTH 16777216

#include "amalgamation/sqlite3.c"
#include "bridge.h"

int pw_sqlite_open(const char *name, pw_sqlite3 **db) {
  int flags = SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_URI |
              SQLITE_OPEN_FULLMUTEX;
  int rc = sqlite3_open_v2(name, (sqlite3 **)db, flags, NULL);
  if (*db != NULL) sqlite3_extended_result_codes((sqlite3 *)*db, 1);
  return rc;
}
int pw_sqlite_close(pw_sqlite3 *db) { return sqlite3_close_v2((sqlite3 *)db); }
const char *pw_sqlite_errmsg(pw_sqlite3 *db) { return sqlite3_errmsg((sqlite3 *)db); }
int pw_sqlite_extended_errcode(pw_sqlite3 *db) { return sqlite3_extended_errcode((sqlite3 *)db); }
void pw_sqlite_interrupt(pw_sqlite3 *db) { sqlite3_interrupt((sqlite3 *)db); }
int pw_sqlite_busy_timeout(pw_sqlite3 *db, int ms) { return sqlite3_busy_timeout((sqlite3 *)db, ms); }
int pw_sqlite_prepare(pw_sqlite3 *db, const char *sql, int n, pw_sqlite3_stmt **stmt) {
  return sqlite3_prepare_v3((sqlite3 *)db, sql, n, SQLITE_PREPARE_PERSISTENT,
                            (sqlite3_stmt **)stmt, NULL);
}
int pw_sqlite_step(pw_sqlite3_stmt *s) { return sqlite3_step((sqlite3_stmt *)s); }
int pw_sqlite_reset(pw_sqlite3_stmt *s) { return sqlite3_reset((sqlite3_stmt *)s); }
int pw_sqlite_clear_bindings(pw_sqlite3_stmt *s) { return sqlite3_clear_bindings((sqlite3_stmt *)s); }
int pw_sqlite_finalize(pw_sqlite3_stmt *s) { return sqlite3_finalize((sqlite3_stmt *)s); }
int pw_sqlite_bind_parameter_count(pw_sqlite3_stmt *s) { return sqlite3_bind_parameter_count((sqlite3_stmt *)s); }
int pw_sqlite_bind_parameter_index(pw_sqlite3_stmt *s, const char *name) { return sqlite3_bind_parameter_index((sqlite3_stmt *)s, name); }
int pw_sqlite_bind_null(pw_sqlite3_stmt *s, int i) { return sqlite3_bind_null((sqlite3_stmt *)s, i); }
int pw_sqlite_bind_int64(pw_sqlite3_stmt *s, int i, int64_t v) { return sqlite3_bind_int64((sqlite3_stmt *)s, i, v); }
int pw_sqlite_bind_double(pw_sqlite3_stmt *s, int i, double v) { return sqlite3_bind_double((sqlite3_stmt *)s, i, v); }
int pw_sqlite_bind_text(pw_sqlite3_stmt *s, int i, const char *v, int n) { return sqlite3_bind_text((sqlite3_stmt *)s, i, v, n, SQLITE_TRANSIENT); }
int pw_sqlite_bind_blob(pw_sqlite3_stmt *s, int i, const void *v, int n) {
  if (n == 0) return sqlite3_bind_zeroblob((sqlite3_stmt *)s, i, 0);
  return sqlite3_bind_blob((sqlite3_stmt *)s, i, v, n, SQLITE_TRANSIENT);
}
int pw_sqlite_column_count(pw_sqlite3_stmt *s) { return sqlite3_column_count((sqlite3_stmt *)s); }
const char *pw_sqlite_column_name(pw_sqlite3_stmt *s, int c) { return sqlite3_column_name((sqlite3_stmt *)s, c); }
const char *pw_sqlite_column_decltype(pw_sqlite3_stmt *s, int c) { return sqlite3_column_decltype((sqlite3_stmt *)s, c); }
int pw_sqlite_column_type(pw_sqlite3_stmt *s, int c) { return sqlite3_column_type((sqlite3_stmt *)s, c); }
int64_t pw_sqlite_column_int64(pw_sqlite3_stmt *s, int c) { return sqlite3_column_int64((sqlite3_stmt *)s, c); }
double pw_sqlite_column_double(pw_sqlite3_stmt *s, int c) { return sqlite3_column_double((sqlite3_stmt *)s, c); }
const unsigned char *pw_sqlite_column_text(pw_sqlite3_stmt *s, int c) { return sqlite3_column_text((sqlite3_stmt *)s, c); }
const void *pw_sqlite_column_blob(pw_sqlite3_stmt *s, int c) { return sqlite3_column_blob((sqlite3_stmt *)s, c); }
int pw_sqlite_column_bytes(pw_sqlite3_stmt *s, int c) { return sqlite3_column_bytes((sqlite3_stmt *)s, c); }
int64_t pw_sqlite_changes(pw_sqlite3 *db) { return sqlite3_changes64((sqlite3 *)db); }
int64_t pw_sqlite_last_insert_rowid(pw_sqlite3 *db) { return sqlite3_last_insert_rowid((sqlite3 *)db); }
const char *pw_sqlite_libversion(void) { return sqlite3_libversion(); }

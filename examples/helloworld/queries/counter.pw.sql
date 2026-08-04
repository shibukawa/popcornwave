package queries

type AccessCounter { count: int }

export statement IncrementAccess(): sql.one<AccessCounter> {
  INSERT INTO access_counter (id, count)
  VALUES (1, 1)
  ON CONFLICT(id) DO UPDATE SET count = access_counter.count + 1
  RETURNING count
}

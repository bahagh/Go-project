-- queries.sql

-- name: InsertTask :one
INSERT INTO tasks (type, value, state)
VALUES ($1, $2, 'received')
RETURNING *;

-- name: UpdateTaskState :exec
UPDATE tasks
SET state = $2, last_update_time = NOW()
WHERE id = $1;

-- name: GetPendingTasks :many
SELECT * FROM tasks WHERE state = 'received' ORDER BY creation_time LIMIT $1;

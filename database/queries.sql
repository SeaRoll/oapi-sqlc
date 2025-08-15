-- name: CreateBook :one
INSERT INTO books (title, author, published_date) VALUES ($1, $2, $3) RETURNING *;
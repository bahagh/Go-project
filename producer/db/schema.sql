-- schema.sql
CREATE TABLE tasks (
    id SERIAL PRIMARY KEY,
    type INT NOT NULL CHECK (type BETWEEN 0 AND 9),
    value INT NOT NULL CHECK (value BETWEEN 0 AND 99),
    state TEXT NOT NULL CHECK (state IN ('received', 'processing', 'done')),
    creation_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

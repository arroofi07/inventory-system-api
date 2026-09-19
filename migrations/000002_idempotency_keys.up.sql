CREATE TABLE IF NOT EXISTS idempotency_keys (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    scope         VARCHAR(64) NOT NULL,
    idem_key      VARCHAR(128) NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    resource_id   BIGINT UNSIGNED NULL,
    request_hash  CHAR(64) NOT NULL,
    response_json JSON NOT NULL,
    status_code   INT NOT NULL DEFAULT 200,
    created_at    DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_idempotency_scope_user_key (scope, user_id, idem_key),
    KEY idx_idempotency_created (created_at),
    CONSTRAINT fk_idempotency_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

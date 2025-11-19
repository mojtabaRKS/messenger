CREATE TABLE credits
(
    id              BIGSERIAL PRIMARY KEY,
    user_id         TEXT UNIQUE NOT NULL,
    credit_balance  BIGINT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE plans
(
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT,
    price       INTEGER     NOT NULL,
    priority    INTEGER     NOT NULL DEFAULT 0,
    api_key     TEXT        NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_plans_api_key ON plans (api_key);
CREATE INDEX idx_plans_priority ON plans (priority);

INSERT INTO plans (name, description, price, priority, api_key)
VALUES ('free', 'free_plan', 0, 1, 'da20bfca-2625-4afc-87a1-528c777eaa07'),
       ('pro', 'pro_plan', 100000, 2, '1c6f516a-dc99-4d76-8ab7-2c53b0c2da41'),
       ('enterprise', 'enterprise_plan', 10000000, 3, '8422f6ac-a32e-4903-8edf-bc951c70ef7f');


CREATE TABLE plans (
     id                  BIGSERIAL PRIMARY KEY,
     name                TEXT NOT NULL,
     description         TEXT NULL,
     price               BIGINT NOT NULL,
     api_key             TEXT NOT NULL,
     created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
     updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO plans (name, description, price, api_key) VALUES ('Free', 'Free plan', 0, '18fecc28-65c4-4fb6-a4d1-8b4059c94976');
INSERT INTO plans (name, description, price, api_key) VALUES ('Pro', 'Pro plan', 10000, 'b9748a9a-ba37-41b1-a2ed-2644efadbd5d');
INSERT INTO plans (name, description, price, api_key) VALUES ('Enterprise', 'Enterprise plan', 100000, '7632944c-2098-4f24-91c4-6d0bf9f77bf7');
CREATE TABLE IF NOT EXISTS adventurers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  rank TEXT NOT NULL,
  guild TEXT NOT NULL,
  region TEXT NOT NULL,
  coverage_status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS providers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  provider_type TEXT NOT NULL,
  tier_rank TEXT NOT NULL,
  region TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  sender_id TEXT NOT NULL,
  receiver_id TEXT NOT NULL,
  payload JSONB NOT NULL,
  raw_x12 TEXT,
  related_id TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS raw_x12 TEXT;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS related_id TEXT;

CREATE TABLE IF NOT EXISTS claims (
  id TEXT PRIMARY KEY,
  adventurer_id TEXT NOT NULL REFERENCES adventurers(id),
  provider_id TEXT NOT NULL REFERENCES providers(id),
  incident_severity TEXT NOT NULL,
  transaction_id TEXT REFERENCES transactions(id),
  amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS enrollments (
  id TEXT PRIMARY KEY,
  adventurer_id TEXT NOT NULL REFERENCES adventurers(id),
  transaction_id TEXT NOT NULL REFERENCES transactions(id),
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS premium_payments (
  id TEXT PRIMARY KEY,
  adventurer_id TEXT NOT NULL REFERENCES adventurers(id),
  transaction_id TEXT NOT NULL REFERENCES transactions(id),
  amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth_requests (
  id TEXT PRIMARY KEY,
  adventurer_id TEXT NOT NULL REFERENCES adventurers(id),
  provider_id TEXT NOT NULL REFERENCES providers(id),
  transaction_id TEXT NOT NULL REFERENCES transactions(id),
  service_type TEXT NOT NULL,
  incident_severity TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inbound_messages (
  id TEXT PRIMARY KEY,
  content_type TEXT NOT NULL,
  transaction_type TEXT,
  raw_payload TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT,
  downstream_status INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_adventurers_coverage_status ON adventurers(coverage_status);
CREATE INDEX IF NOT EXISTS idx_providers_region ON providers(region);
CREATE INDEX IF NOT EXISTS idx_providers_tier_rank ON providers(tier_rank);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_related_id ON transactions(related_id);
CREATE INDEX IF NOT EXISTS idx_claims_adventurer_id ON claims(adventurer_id);
CREATE INDEX IF NOT EXISTS idx_claims_provider_id ON claims(provider_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON claims(status);
CREATE INDEX IF NOT EXISTS idx_enrollments_adventurer_id ON enrollments(adventurer_id);
CREATE INDEX IF NOT EXISTS idx_premium_payments_adventurer_id ON premium_payments(adventurer_id);
CREATE INDEX IF NOT EXISTS idx_auth_requests_adventurer_id ON auth_requests(adventurer_id);
CREATE INDEX IF NOT EXISTS idx_auth_requests_provider_id ON auth_requests(provider_id);
CREATE INDEX IF NOT EXISTS idx_inbound_messages_transaction_type ON inbound_messages(transaction_type);
CREATE INDEX IF NOT EXISTS idx_inbound_messages_status ON inbound_messages(status);

INSERT INTO providers (id, name, provider_type, tier_rank, region) VALUES
  ('provider-greenstone-roadside', 'Greenstone Roadside Clinic', 'Clinic', 'Iron', 'Greenstone'),
  ('provider-westbridge-outpost', 'Westbridge Outpost', 'Outpost', 'Iron', 'Greenstone'),
  ('provider-yaresh-regional', 'Yaresh Regional Healing Centre', 'Clinic', 'Silver', 'Yaresh'),
  ('provider-jungle-wardens', 'Jungle Warden''s Guild', 'Clinic', 'Silver', 'Yaresh'),
  ('provider-rimaros-hospital', 'Rimaros City Hospital', 'Clinic', 'Gold', 'Rimaros'),
  ('provider-vitesse-temple', 'Temple of the Healer, Vitesse', 'Temple', 'Diamond', 'Vitesse')
ON CONFLICT (name) DO NOTHING;

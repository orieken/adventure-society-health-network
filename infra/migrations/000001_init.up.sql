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

CREATE TABLE IF NOT EXISTS trading_partners (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  sender_id TEXT NOT NULL UNIQUE,
  receiver_id TEXT NOT NULL,
  allowed_transaction_types TEXT NOT NULL,
  route_target TEXT NOT NULL DEFAULT 'payer-core',
  status TEXT NOT NULL DEFAULT 'active',
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
  allowed_amount_cents BIGINT NOT NULL DEFAULT 0,
  paid_amount_cents BIGINT NOT NULL DEFAULT 0,
  patient_responsibility_cents BIGINT NOT NULL DEFAULT 0,
  adjustment_amount_cents BIGINT NOT NULL DEFAULT 0,
  adjustment_reason TEXT,
  denial_reason TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE claims ADD COLUMN IF NOT EXISTS allowed_amount_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE claims ADD COLUMN IF NOT EXISTS paid_amount_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE claims ADD COLUMN IF NOT EXISTS patient_responsibility_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE claims ADD COLUMN IF NOT EXISTS adjustment_amount_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE claims ADD COLUMN IF NOT EXISTS adjustment_reason TEXT;
ALTER TABLE claims ADD COLUMN IF NOT EXISTS denial_reason TEXT;

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
  partner_id TEXT REFERENCES trading_partners(id),
  content_type TEXT NOT NULL,
  transaction_type TEXT,
  raw_payload TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT,
  downstream_status INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE inbound_messages ADD COLUMN IF NOT EXISTS partner_id TEXT REFERENCES trading_partners(id);

CREATE TABLE IF NOT EXISTS transaction_jobs (
  id TEXT PRIMARY KEY,
  job_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_adventurers_coverage_status ON adventurers(coverage_status);
CREATE INDEX IF NOT EXISTS idx_providers_region ON providers(region);
CREATE INDEX IF NOT EXISTS idx_providers_tier_rank ON providers(tier_rank);
CREATE INDEX IF NOT EXISTS idx_trading_partners_sender_id ON trading_partners(sender_id);
CREATE INDEX IF NOT EXISTS idx_trading_partners_status ON trading_partners(status);
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
CREATE INDEX IF NOT EXISTS idx_inbound_messages_partner_id ON inbound_messages(partner_id);
CREATE INDEX IF NOT EXISTS idx_inbound_messages_transaction_type ON inbound_messages(transaction_type);
CREATE INDEX IF NOT EXISTS idx_inbound_messages_status ON inbound_messages(status);
CREATE INDEX IF NOT EXISTS idx_transaction_jobs_due ON transaction_jobs(status, run_after);
CREATE INDEX IF NOT EXISTS idx_transaction_jobs_entity_id ON transaction_jobs(entity_id);

INSERT INTO providers (id, name, provider_type, tier_rank, region) VALUES
  ('provider-greenstone-roadside', 'Greenstone Roadside Clinic', 'Clinic', 'Iron', 'Greenstone'),
  ('provider-westbridge-outpost', 'Westbridge Outpost', 'Outpost', 'Iron', 'Greenstone'),
  ('provider-yaresh-regional', 'Yaresh Regional Healing Centre', 'Clinic', 'Silver', 'Yaresh'),
  ('provider-jungle-wardens', 'Jungle Warden''s Guild', 'Clinic', 'Silver', 'Yaresh'),
  ('provider-rimaros-hospital', 'Rimaros City Hospital', 'Clinic', 'Gold', 'Rimaros'),
  ('provider-vitesse-temple', 'Temple of the Healer, Vitesse', 'Temple', 'Diamond', 'Vitesse')
ON CONFLICT (name) DO NOTHING;

INSERT INTO trading_partners (id, name, sender_id, receiver_id, allowed_transaction_types, route_target, status) VALUES
  ('tp-greenstone-guild', 'Greenstone Employer Guild', 'partner-greenstone', 'Adventure Society', '834,820', 'payer-core', 'active'),
  ('tp-vitesse-temple', 'Temple of the Healer, Vitesse', 'provider-vitesse-temple', 'Adventure Society', '270,275,276,278,837', 'payer-core', 'active'),
  ('tp-rimaros-hospital', 'Rimaros City Hospital', 'provider-rimaros-hospital', 'Adventure Society', '270,275,276,278,837', 'payer-core', 'active')
ON CONFLICT (sender_id) DO UPDATE SET
  name = EXCLUDED.name,
  receiver_id = EXCLUDED.receiver_id,
  allowed_transaction_types = EXCLUDED.allowed_transaction_types,
  route_target = EXCLUDED.route_target,
  status = EXCLUDED.status;

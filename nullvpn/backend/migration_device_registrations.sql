-- Migration: device_registrations table for sing-box NullVPN provisioning
-- Apply: psql -U postgres -d nullvpn -f migration_device_registrations.sql

CREATE TABLE IF NOT EXISTS device_registrations (
  device_id   VARCHAR(64)  PRIMARY KEY,   -- SHA-256(ANDROID_ID + pkg), 32 hex chars
  public_key  VARCHAR(64)  NOT NULL,      -- Curve25519 base64 (future: for e2e config encryption)
  user_uuid   UUID         NOT NULL,      -- VLESS UUID for sing-box auth
  tier        VARCHAR(16)  NOT NULL DEFAULT 'trial',
  status      VARCHAR(20)  NOT NULL DEFAULT 'pending',
  registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_device_registrations_tier   ON device_registrations(tier);
CREATE INDEX IF NOT EXISTS idx_device_registrations_status ON device_registrations(status);

COMMENT ON TABLE device_registrations IS
  'NullVPN APK device registry — maps device fingerprint to sing-box VLESS UUID';

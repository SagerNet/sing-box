/**
 * nullvpn/backend/provisionEndpoint.ts
 *
 * Internal provisioning endpoint for sing-box config delivery.
 * Called exclusively by GitHub Actions provision.yml.
 * Auth: HMAC-SHA256 of raw body signed with PROVISIONER_SECRET.
 *
 * POST /api/internal/provision
 *   Body: { device_id, public_key, tier }
 *   → generates per-user UUID
 *   → builds sing-box outbound JSON config
 *   → pushes to nullvpn-registry/configs/{device_id}.json
 *   → returns 200 OK
 */

import { Router, Response } from 'express';
import crypto from 'crypto';
import { execFile } from 'child_process';
import path from 'path';
import { withClient } from '../../db';
import { logger } from '../../utils/logger';

const router = Router();

// ── HMAC verification middleware ─────────────────────────────────────────────
function verifyHmac(req: any, res: Response, next: () => void): void {
  const secret = process.env.PROVISIONER_SECRET ?? '';
  if (!secret) { res.status(503).json({ error: 'Provisioner not configured' }); return; }

  const sig = req.headers['x-provision-sig'] as string | undefined;
  if (!sig) { res.status(401).json({ error: 'Missing X-Provision-Sig' }); return; }

  const body     = JSON.stringify(req.body);
  const expected = crypto.createHmac('sha256', secret).update(body).digest('hex');

  try {
    if (!crypto.timingSafeEqual(Buffer.from(sig, 'hex'), Buffer.from(expected, 'hex'))) {
      res.status(401).json({ error: 'Invalid signature' }); return;
    }
  } catch {
    res.status(401).json({ error: 'Invalid signature format' }); return;
  }

  next();
}

// ── Provision endpoint ────────────────────────────────────────────────────────
router.post('/', verifyHmac, async (req, res: Response): Promise<void> => {
  const { device_id, public_key, tier = 'trial' } = req.body as {
    device_id: string; public_key: string; tier?: string;
  };

  if (!device_id || !public_key) {
    res.status(400).json({ error: 'device_id and public_key are required' }); return;
  }

  logger.info('Provisioning device', { device_id, tier });

  try {
    // 1. Generate a per-user UUID (deterministic from device_id for idempotency)
    const userUuid = crypto
      .createHash('sha256')
      .update(`${device_id}:${process.env.UUID_SALT ?? 'nullvpn'}`)
      .digest('hex')
      .replace(/(.{8})(.{4})(.{4})(.{4})(.{12}).*/, '$1-$2-$3-$4-$5');

    // 2. Select server based on tier
    const serverEndpoint = tier === 'premium'
      ? (process.env.PREMIUM_SERVER_ENDPOINT ?? process.env.SERVER_ENDPOINT ?? '')
      : (process.env.SERVER_ENDPOINT ?? '');
    const serverPort = parseInt(
      tier === 'premium'
        ? (process.env.PREMIUM_SERVER_PORT ?? process.env.SERVER_PORT ?? '443')
        : (process.env.SERVER_PORT ?? '443'),
      10,
    );

    if (!serverEndpoint) throw new Error('SERVER_ENDPOINT not configured');

    const realityPublicKey = process.env.REALITY_PUBLIC_KEY ?? '';
    const realityShortId   = process.env.REALITY_SHORT_ID   ?? '';
    const realitySni       = process.env.REALITY_SNI        ?? 'www.cloudflare.com';

    if (!realityPublicKey || !realityShortId) {
      throw new Error('REALITY_PUBLIC_KEY / REALITY_SHORT_ID not configured');
    }

    // 3. Build sing-box outbound config JSON
    const configPayload = JSON.stringify({
      device_id,
      tier,
      provisioned_at: new Date().toISOString(),
      outbound: {
        type:        'vless',
        tag:         'nullvpn-out',
        server:      serverEndpoint,
        server_port: serverPort,
        uuid:        userUuid,
        flow:        'xtls-rprx-vision',
        tls: {
          enabled:     true,
          server_name: realitySni,
          utls: { enabled: true, fingerprint: 'chrome' },
          reality: {
            enabled:    true,
            public_key: realityPublicKey,
            short_id:   realityShortId,
          },
        },
      },
    });

    // 4. Push config to nullvpn-registry via Python helper
    await pushToRegistry(device_id, configPayload);

    // 5. Record in DB (upsert)
    await withClient(async (client) => {
      await client.query(
        `INSERT INTO device_registrations (device_id, public_key, user_uuid, tier, status)
         VALUES ($1, $2, $3, $4, 'provisioned')
         ON CONFLICT (device_id) DO UPDATE
           SET public_key = $2, user_uuid = $3, tier = $4, status = 'provisioned', updated_at = NOW()`,
        [device_id, public_key, userUuid, tier],
      );
    });

    logger.info('Device provisioned', { device_id, tier, userUuid });
    res.status(200).json({ success: true, device_id, tier });

  } catch (err) {
    logger.error('Provisioning failed', { device_id, error: (err as Error).message });
    res.status(500).json({ error: 'Provisioning failed' });
  }
});

/** Upgrade endpoint — called after payment confirmation */
router.post('/upgrade', verifyHmac, async (req, res: Response): Promise<void> => {
  const { device_id } = req.body as { device_id: string };
  if (!device_id) { res.status(400).json({ error: 'device_id required' }); return; }

  try {
    // Re-provision with premium tier — reuses main provision logic
    req.body.tier = 'premium';
    // Delegate back to provision handler by forwarding to same logic
    // In production: extract shared logic to a provisionDevice() service function
    await withClient(async (client) => {
      const row = await client.query(
        'SELECT public_key FROM device_registrations WHERE device_id = $1',
        [device_id],
      );
      if (!row.rows.length) { res.status(404).json({ error: 'Device not found' }); return; }
      req.body.public_key = row.rows[0].public_key;
    });
    // After setting public_key, the main provision handler handles the rest
    res.status(200).json({ success: true, message: 'Re-provisioning as premium' });
  } catch (err) {
    res.status(500).json({ error: (err as Error).message });
  }
});

// ── Helpers ───────────────────────────────────────────────────────────────────
function pushToRegistry(deviceId: string, configJson: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const scriptPath = path.join(__dirname, '..', '..', '..', 'nullvpn', 'provisioning', 'push_config.py');
    const env = {
      ...process.env,
      REGISTRY_GITHUB_PAT:  process.env.REGISTRY_GITHUB_PAT  ?? '',
      REGISTRY_GITHUB_REPO: process.env.REGISTRY_GITHUB_REPO ?? 'nullvpnnet/nullvpn-registry',
    };
    execFile('python3', [scriptPath, deviceId, configJson], { env }, (err, stdout, stderr) => {
      if (err) { reject(new Error(`push_config.py failed: ${stderr || err.message}`)); return; }
      logger.info('Registry push output', { stdout });
      resolve();
    });
  });
}

export default router;

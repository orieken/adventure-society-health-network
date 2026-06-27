#!/usr/bin/env bash
set -euo pipefail

ASHN_API_URL="${ASHN_API_URL:-http://localhost:8080}"

node - "$ASHN_API_URL" <<'NODE'
const apiUrl = process.argv[2];

async function request(path, options = {}) {
  const response = await fetch(`${apiUrl}${path}`, {
    ...options,
    headers: {
      "content-type": "application/json",
      ...(options.headers ?? {})
    }
  });
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(`${options.method ?? "GET"} ${path} failed: ${response.status} ${JSON.stringify(payload)}`);
  }
  return payload;
}

async function post(path, body) {
  return request(path, { method: "POST", body: JSON.stringify(body) });
}

function line(title, detail = "") {
  console.log(`\n=== ${title} ===`);
  if (detail) console.log(detail);
}

function transactionSummary(envelope) {
  const transactions = envelope.transactions ?? (envelope.transaction ? [envelope.transaction] : []);
  return transactions.map((tx) => `${tx.type}:${tx.status}`).join(", ");
}

line("ASHN Demo Flow", `API: ${apiUrl}`);

const health = await request("/v1/health");
console.log("Health:", health.data);

const providers = await request("/v1/providers");
const provider = providers.data.find((item) => item.id === "provider-vitesse-temple") ?? providers.data[0];
console.log("Provider:", `${provider.name} (${provider.tierRank}, ${provider.region})`);

line("1. Enrollment");
const enrollment = await post("/v1/adventurers", {
  name: `Farros Demo ${new Date().toISOString().slice(11, 19)}`,
  rank: "Iron",
  guild: "Grim Foundations",
  region: "Greenstone"
});
console.log(enrollment.lore);
console.log("Transactions:", transactionSummary(enrollment));
console.log("Adventurer:", enrollment.data.id);

line("2. Eligibility");
const eligibility = await post("/v1/eligibility", {
  adventurerId: enrollment.data.id,
  providerId: provider.id
});
console.log(eligibility.lore);
console.log("Transactions:", transactionSummary(eligibility));
console.log("Eligible:", eligibility.data.eligible);

line("3. Prior Authorization");
const auth = await post("/v1/auth-requests", {
  adventurerId: enrollment.data.id,
  providerId: provider.id,
  serviceType: "resurrection",
  incidentSeverity: "Diamond"
});
console.log(auth.lore);
console.log("Transactions:", transactionSummary(auth));
console.log("Decision:", auth.data.authorizationStatus);

line("4. Claim Submission");
const claim = await post("/v1/claims", {
  adventurerId: enrollment.data.id,
  providerId: provider.id,
  incidentSeverity: "Awakened",
  amountCents: 125000
});
console.log(claim.lore);
console.log("Transactions:", transactionSummary(claim));
console.log("Claim:", claim.data.id);

line("5. Claim Status");
const status = await request(`/v1/claims/${claim.data.id}/status`);
console.log(status.lore);
console.log("Transactions:", transactionSummary(status));
console.log("Status:", status.data.status);

line("6. Payment");
const payment = await post(`/v1/claims/${claim.data.id}/payment`, {
  paymentAmountCents: 100000
});
console.log(payment.lore);
console.log("Transactions:", transactionSummary(payment));
console.log("Final claim status:", payment.data.status);

line("Dashboard");
console.log("Open http://localhost:9300 to inspect the Society ledger.");
NODE

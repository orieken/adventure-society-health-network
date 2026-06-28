import React, { FormEvent, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type Provider = {
  id: string;
  name: string;
  providerType: string;
  tierRank: string;
  region: string;
};

type Adventurer = {
  id: string;
  name: string;
  rank: string;
  guild: string;
  region: string;
  coverageStatus: string;
};

type Claim = {
  id: string;
  adventurerId: string;
  providerId: string;
  incidentSeverity: string;
  transactionId: string;
  amountCents: number;
  status: string;
};

type Envelope<T = unknown> = {
  data?: T;
  lore?: string;
  transaction?: Transaction;
  transactions?: Transaction[];
  error?: string;
};

type Transaction = {
  id: string;
  type: string;
  status: string;
  senderId: string;
  receiverId: string;
  payload: unknown;
  createdAt: string;
};

const apiUrl = import.meta.env.VITE_ASHN_API_URL ?? "http://localhost:8080";

function App() {
  const [health, setHealth] = useState<Envelope<Record<string, string>> | null>(null);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [selectedProviderId, setSelectedProviderId] = useState("provider-vitesse-temple");
  const [adventurer, setAdventurer] = useState<Adventurer | null>(null);
  const [claim, setClaim] = useState<Claim | null>(null);
  const [recentAdventurers, setRecentAdventurers] = useState<Adventurer[]>([]);
  const [recentClaims, setRecentClaims] = useState<Claim[]>([]);
  const [recentTransactions, setRecentTransactions] = useState<Transaction[]>([]);
  const [selectedClaim, setSelectedClaim] = useState<Claim | null>(null);
  const [selectedTransaction, setSelectedTransaction] = useState<Transaction | null>(null);
  const [events, setEvents] = useState<Envelope[]>([]);
  const [busy, setBusy] = useState(false);

  const selectedProvider = useMemo(
    () => providers.find((provider) => provider.id === selectedProviderId),
    [providers, selectedProviderId]
  );

  useEffect(() => {
    void refresh();
  }, []);

  async function refresh() {
    const [healthResult, providersResult, adventurersResult, claimsResult, transactionsResult] = await Promise.all([
      request<Record<string, string>>("/v1/health"),
      request<Provider[]>("/v1/providers"),
      request<Adventurer[]>("/v1/adventurers?limit=10"),
      request<Claim[]>("/v1/claims?limit=10"),
      request<Transaction[]>("/v1/transactions?limit=25")
    ]);
    setHealth(healthResult);
    setProviders(providersResult.data ?? []);
    setRecentAdventurers(adventurersResult.data ?? []);
    setRecentClaims(claimsResult.data ?? []);
    setRecentTransactions(transactionsResult.data ?? []);
    if (providersResult.lore) {
      pushEvent(providersResult);
    }
  }

  async function request<T>(path: string, init?: RequestInit): Promise<Envelope<T>> {
    const response = await fetch(`${apiUrl}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {})
      }
    });
    return (await response.json()) as Envelope<T>;
  }

  function pushEvent(event: Envelope) {
    setEvents((current) => [event, ...current].slice(0, 8));
  }

  async function enroll(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    const form = new FormData(event.currentTarget);
    const result = await request<Adventurer>("/v1/adventurers", {
      method: "POST",
      body: JSON.stringify({
        name: form.get("name"),
        rank: form.get("rank"),
        guild: form.get("guild"),
        region: form.get("region")
      })
    });
    setAdventurer(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function checkEligibility() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request("/v1/eligibility", {
      method: "POST",
      body: JSON.stringify({ adventurerId: adventurer.id, providerId: selectedProviderId })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function requestAuth() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request("/v1/auth-requests", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        serviceType: "resurrection",
        incidentSeverity: "Diamond"
      })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function submitClaim() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Claim>("/v1/claims", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        incidentSeverity: "Awakened",
        amountCents: 125000
      })
    });
    setClaim(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function payClaim() {
    if (!claim) return;
    setBusy(true);
    const result = await request<Claim>(`/v1/claims/${claim.id}/payment`, {
      method: "POST",
      body: JSON.stringify({ paymentAmountCents: 100000 })
    });
    setClaim(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function openClaimDetail(claimId: string) {
    setBusy(true);
    const result = await request<Claim>(`/v1/claims/${claimId}`);
    if (result.data) {
      setSelectedClaim(result.data);
      setSelectedTransaction(null);
    }
    setBusy(false);
  }

  async function openTransactionDetail(transactionId: string) {
    setBusy(true);
    const result = await request<Transaction>(`/v1/transactions/${transactionId}`);
    const transaction = result.transaction ?? result.data;
    if (transaction) {
      setSelectedTransaction(transaction);
      setSelectedClaim(null);
    }
    setBusy(false);
  }

  return (
    <main>
      <section className="hero">
        <div>
          <p className="eyebrow">Adventure Society Health Network</p>
          <h1>ASHN Transaction Dashboard</h1>
          <p>
            Enroll adventurers, verify eligibility, request resurrection authorization,
            and send claim/remittance transactions through the gateway.
          </p>
        </div>
        <div className="status-card">
          <span className="rune">◆</span>
          <h2>Gateway</h2>
          <p>{apiUrl}</p>
          <div className="health-grid">
            {Object.entries(health?.data ?? {}).map(([service, status]) => (
              <span key={service} className={status === "ok" ? "ok" : "bad"}>
                {service}: {status}
              </span>
            ))}
          </div>
        </div>
      </section>

      <section className="stats-grid">
        <MetricCard label="Adventurers" value={recentAdventurers.length} detail="recent registrations" />
        <MetricCard label="Claims" value={recentClaims.length} detail={`${recentClaims.filter((item) => item.status === "Paid").length} paid`} />
        <MetricCard label="Transactions" value={recentTransactions.length} detail="ledger entries loaded" />
      </section>

      <section className="grid">
        <div className="panel">
          <h2>1. Enroll Adventurer</h2>
          <form onSubmit={enroll}>
            <label>
              Name
              <input name="name" defaultValue="Farros" />
            </label>
            <label>
              Rank
              <select name="rank" defaultValue="Iron">
                {["Iron", "Bronze", "Silver", "Gold", "Diamond"].map((rank) => (
                  <option key={rank}>{rank}</option>
                ))}
              </select>
            </label>
            <label>
              Guild
              <input name="guild" defaultValue="Grim Foundations" />
            </label>
            <label>
              Region
              <select name="region" defaultValue="Greenstone">
                {["Greenstone", "Yaresh", "Rimaros", "Vitesse"].map((region) => (
                  <option key={region}>{region}</option>
                ))}
              </select>
            </label>
            <button disabled={busy}>Send 834 Enrollment</button>
          </form>
          {adventurer && (
            <div className="result">
              <strong>{adventurer.name}</strong>
              <span>{adventurer.rank} · {adventurer.coverageStatus}</span>
              <code>{adventurer.id}</code>
            </div>
          )}
        </div>

        <div className="panel">
          <h2>2. Provider + Workflow</h2>
          <label>
            Treating provider
            <select value={selectedProviderId} onChange={(event) => setSelectedProviderId(event.target.value)}>
              {providers.map((provider) => (
                <option value={provider.id} key={provider.id}>{provider.name}</option>
              ))}
            </select>
          </label>
          {selectedProvider && (
            <div className="provider-card">
              <h3>{selectedProvider.name}</h3>
              <p>{selectedProvider.providerType} · {selectedProvider.tierRank} · {selectedProvider.region}</p>
            </div>
          )}
          <div className="actions">
            <button disabled={!adventurer || busy} onClick={checkEligibility}>270 → 271 Eligibility</button>
            <button disabled={!adventurer || busy} onClick={requestAuth}>278 Resurrection Auth</button>
            <button disabled={!adventurer || busy} onClick={submitClaim}>837 Submit Claim</button>
            <button disabled={!claim || busy} onClick={payClaim}>835 Pay Claim</button>
          </div>
          {claim && (
            <div className="result">
              <strong>Claim {claim.status}</strong>
              <span>{claim.incidentSeverity} · ${(claim.amountCents / 100).toLocaleString()}</span>
              <code>{claim.id}</code>
            </div>
          )}
        </div>
      </section>

      <section className="panel ledger">
        <div className="ledger-title">
          <h2>Live Session Events</h2>
          <button onClick={refresh} disabled={busy}>Refresh</button>
        </div>
        {events.length === 0 ? (
          <p className="muted">No transactions yet. The Society scribe is sharpening a quill.</p>
        ) : (
          events.map((event, index) => <LedgerEvent key={index} event={event} />)
        )}
      </section>

      <section className="history-grid">
        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Persisted Transactions</h2>
            <span className="muted">from Postgres</span>
          </div>
          {recentTransactions.length === 0 ? (
            <p className="muted">No persisted transactions yet.</p>
          ) : (
            recentTransactions.map((transaction) => (
              <TransactionRow key={transaction.id} transaction={transaction} onSelect={openTransactionDetail} />
            ))
          )}
        </div>

        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Recent Claims</h2>
            <span className="muted">from Postgres</span>
          </div>
          {recentClaims.length === 0 ? (
            <p className="muted">No claims yet.</p>
          ) : (
            recentClaims.map((item) => <ClaimRow key={item.id} claim={item} onSelect={openClaimDetail} />)
          )}
        </div>

        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Recent Adventurers</h2>
            <span className="muted">from Postgres</span>
          </div>
          {recentAdventurers.length === 0 ? (
            <p className="muted">No adventurers yet.</p>
          ) : (
            recentAdventurers.map((item) => <AdventurerRow key={item.id} adventurer={item} />)
          )}
        </div>
      </section>

      {(selectedClaim || selectedTransaction) && (
        <section className="panel detail-panel">
          <div className="ledger-title">
            <h2>{selectedTransaction ? "Transaction Detail" : "Claim Detail"}</h2>
            <button className="secondary" onClick={() => {
              setSelectedClaim(null);
              setSelectedTransaction(null);
            }}>
              Close
            </button>
          </div>
          {selectedTransaction && (
            <div className="detail-grid">
              <DetailItem label="Type" value={selectedTransaction.type} />
              <DetailItem label="Status" value={selectedTransaction.status} />
              <DetailItem label="Sender" value={selectedTransaction.senderId} />
              <DetailItem label="Receiver" value={selectedTransaction.receiverId} />
              <DetailItem label="Created" value={new Date(selectedTransaction.createdAt).toLocaleString()} />
              <DetailItem label="ID" value={selectedTransaction.id} />
              <pre>{JSON.stringify(selectedTransaction.payload, null, 2)}</pre>
            </div>
          )}
          {selectedClaim && (
            <div className="detail-grid">
              <DetailItem label="Status" value={selectedClaim.status} />
              <DetailItem label="Severity" value={selectedClaim.incidentSeverity} />
              <DetailItem label="Amount" value={`$${(selectedClaim.amountCents / 100).toLocaleString()}`} />
              <DetailItem label="Adventurer" value={selectedClaim.adventurerId} />
              <DetailItem label="Provider" value={selectedClaim.providerId} />
              <DetailItem label="Transaction" value={selectedClaim.transactionId} />
              <DetailItem label="Claim ID" value={selectedClaim.id} />
            </div>
          )}
        </section>
      )}
    </main>
  );
}

function MetricCard({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <p>{detail}</p>
    </div>
  );
}

function LedgerEvent({ event }: { event: Envelope }) {
  const transactions = event.transactions ?? (event.transaction ? [event.transaction] : []);
  return (
    <article className="event">
      <p>{event.lore ?? event.error ?? "Transaction event"}</p>
      <div className="chips">
        {transactions.map((transaction) => (
          <span key={transaction.id}>
            {transaction.type} · {transaction.status}
          </span>
        ))}
      </div>
      <details>
        <summary>Raw payload</summary>
        <pre>{JSON.stringify(event, null, 2)}</pre>
      </details>
    </article>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="detail-item">
      <span>{label}</span>
      <strong>{value || "—"}</strong>
    </div>
  );
}

function TransactionRow({ transaction, onSelect }: { transaction: Transaction; onSelect: (transactionId: string) => void }) {
  return (
    <article className="compact-row clickable" onClick={() => onSelect(transaction.id)}>
      <div>
        <strong>{transaction.type} · {transaction.status}</strong>
        <span>{new Date(transaction.createdAt).toLocaleString()}</span>
      </div>
      <code>{transaction.id}</code>
    </article>
  );
}

function ClaimRow({ claim, onSelect }: { claim: Claim; onSelect: (claimId: string) => void }) {
  return (
    <article className="compact-row clickable" onClick={() => onSelect(claim.id)}>
      <div>
        <strong>{claim.status} · {claim.incidentSeverity}</strong>
        <span>${(claim.amountCents / 100).toLocaleString()} · provider {claim.providerId}</span>
      </div>
      <code>{claim.id}</code>
    </article>
  );
}

function AdventurerRow({ adventurer }: { adventurer: Adventurer }) {
  return (
    <article className="compact-row">
      <div>
        <strong>{adventurer.name}</strong>
        <span>{adventurer.rank} · {adventurer.region} · {adventurer.coverageStatus}</span>
      </div>
      <code>{adventurer.id}</code>
    </article>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

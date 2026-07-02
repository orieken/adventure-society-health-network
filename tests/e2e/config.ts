export type ServiceName = "apiGateway" | "payerCore" | "providerService" | "ediIntake";

export type ServiceContract = {
  key: ServiceName;
  label: string;
  baseURL: string;
  healthPath: string;
  expectedService: string;
};

const trimTrailingSlash = (value: string) => value.replace(/\/+$/, "");

export const serviceUrls: Record<ServiceName, string> = {
  apiGateway: trimTrailingSlash(process.env.ASHN_API_URL ?? "https://ashn-api-gateway.onrender.com"),
  payerCore: trimTrailingSlash(process.env.ASHN_PAYER_CORE_URL ?? "https://ashn-payer-core.onrender.com"),
  providerService: trimTrailingSlash(process.env.ASHN_PROVIDER_SERVICE_URL ?? "https://ashn-provider-service.onrender.com"),
  ediIntake: trimTrailingSlash(process.env.ASHN_EDI_INTAKE_URL ?? "https://ashn-edi-intake.onrender.com")
};

export const dashboardUrl = process.env.ASHN_DASHBOARD_URL ? trimTrailingSlash(process.env.ASHN_DASHBOARD_URL) : "";

export const runMutatingE2E = process.env.ASHN_RUN_MUTATING_E2E === "1";

export const services: ServiceContract[] = [
  {
    key: "apiGateway",
    label: "API Gateway",
    baseURL: serviceUrls.apiGateway,
    healthPath: "/v1/health",
    expectedService: "api-gateway"
  },
  {
    key: "payerCore",
    label: "Payer Core",
    baseURL: serviceUrls.payerCore,
    healthPath: "/health",
    expectedService: "payer-core"
  },
  {
    key: "providerService",
    label: "Provider Service",
    baseURL: serviceUrls.providerService,
    healthPath: "/health",
    expectedService: "provider-service"
  },
  {
    key: "ediIntake",
    label: "EDI Intake",
    baseURL: serviceUrls.ediIntake,
    healthPath: "/health",
    expectedService: "edi-intake"
  }
];

export const uniqueDemoName = (prefix: string) => {
  const timestamp = new Date().toISOString().replace(/[-:.TZ]/g, "");
  return `${prefix} ${timestamp}`;
};

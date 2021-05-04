export const isError = (r: any | ErrorResponse): r is ErrorResponse => {
    return (r as ErrorResponse).error !== undefined;
};

export interface Configuration {
    checks: Check[]
    values: Value[]
};

interface Check {
    name: string
    description: string
    dns?: DNSCheck
    systemd?: SystemdCheck
};

interface DNSCheck {
    value: string
};

interface SystemdCheck {
    unit: string
    description: string
};

interface Value {
    name: string
    description: string
    secret: boolean
};

interface ErrorResponse {
    error: string
};

interface DNSResponse {
};

export enum SystemdResult {
    Failed = "failed",
    Success = "success",
}

export enum SystemdSubState {
    Dead = "dead",
    Start = "start",
}

interface JoinResponse {
    result: SystemdResult
    subState: SystemdSubState
};

interface LogEntry {
    MESSAGE: string
    __REALTIME_TIMESTAMP: string
};

interface LinkResponse {
    addresses: string[]
    state: string
};

interface OnboardResponse {
};

interface Client {
    dns(endpoint: string): Promise<DNSResponse|ErrorResponse>
    link(): Promise<LinkResponse|ErrorResponse>
    log(name: string, append: (logs: LogEntry[]) => void): () => void
    onboard(request: string): Promise<OnboardResponse|ErrorResponse>
    systemd(unit: string): Promise<JoinResponse|ErrorResponse>
};

export const client: Client = {
    dns: (endpoint: string): Promise<DNSResponse|ErrorResponse> => {
        return fetch("/api/v1/status/dns?" + new URLSearchParams({endpoint})).then(r => {
            if (r.ok) {
                return {};
            }
            return r.json().then((rr: ErrorResponse) => {
                if (Math.floor(r.status) === 4) {
                    throw new Error((rr as ErrorResponse).error);
                }
                return rr;
            });
        });
    },
    systemd: (unit: string): Promise<JoinResponse|ErrorResponse> => {
        return fetch("/api/v1/status/systemd?"+ new URLSearchParams({"unit": unit})).then(r => {
            return r.json().then((rr: JoinResponse|ErrorResponse) => {
                if (r.ok) {
                    return rr;
                }
                if (Math.floor(r.status) === 4) {
                    throw new Error((rr as ErrorResponse).error);
                }
                return rr;
            });
        });
    },
    link: (): Promise<LinkResponse|ErrorResponse> => {
        return fetch("/api/v1/status/link").then(r => {
            return r.json().then((rr: LinkResponse|ErrorResponse) => {
                if (r.ok) {
                    return rr;
                }
                if (Math.floor(r.status) === 4) {
                    throw new Error((rr as ErrorResponse).error);
                }
                return rr;
            });
        });
    },
    log: (name: string, append: (logs: LogEntry[]) => void): () => void => {
        const es = new EventSource("/api/v1/log/" + name);
        es.onmessage = (e: MessageEvent): void => {
            append(JSON.parse("[" + (e.data as string).replace("\n", ",") + "]"));
        }
        return es.close
    },
    onboard: (request: string): Promise<OnboardResponse|ErrorResponse> => {
        return fetch("/api/v1/onboard", {
            method: "POST",
            headers: {
              "Content-Type": "application/json"
            },
            body: request
          }).then(r => {
            if (r.ok) {
                return {};
            }
            return r.json().then((rr: ErrorResponse) => {
                if (Math.floor(r.status) === 4) {
                    throw new Error((rr as ErrorResponse).error);
                }
                return rr;
            });
        });
    },
}

export default client;

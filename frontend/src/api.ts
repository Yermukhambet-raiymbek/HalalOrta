// Typed client for the Go API. Shapes mirror backend/internal/store and
// backend/internal/httpapi. The session lives in an HttpOnly cookie that the
// browser sends automatically; JavaScript never sees it.

export const RELATIONSHIPS = ["Mom", "Dad", "Grandma", "Grandpa", "Other"] as const;
export type Relationship = (typeof RELATIONSHIPS)[number];

export type Member = {
  id: string;
  name: string;
  relationship: Relationship;
  is_owner: boolean;
};

export type FamilyRef = {
  id: string;
  name: string;
};

/** A member created in this browser, i.e. one the header can switch to. */
export type DeviceMember = {
  id: string;
  name: string;
  relationship: Relationship;
  family_name: string;
};

export type Me = {
  member: Member;
  family: FamilyRef;
  device_members: DeviceMember[];
};

export type FamilyDetails = {
  family: FamilyRef;
  members: Member[];
  invite: { token: string; path: string };
};

export type Invite = {
  family_name: string;
  invited_by: string;
};

/** State of one request, for loading and error UI. */
export type Load<T> =
  | { status: "loading" }
  | { status: "error"; error: unknown }
  | { status: "ready"; data: T };

/** An error reply in the API's {"error": {"code", "message"}} format. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers:
        body === undefined
          ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network_error", "Can't reach the server. Check your connection and try again.");
  }

  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(
      res.status,
      err?.code ?? "http_error",
      err?.message ?? `The server returned an error (${res.status}). Please try again.`,
    );
  }
  if (data === null) {
    throw new ApiError(res.status, "bad_response", "The server sent an unexpected reply. Please try again.");
  }
  return data as T;
}

export const api = {
  me: () => request<Me>("GET", "/api/me"),
  family: () => request<FamilyDetails>("GET", "/api/family"),
  invite: (token: string) => request<Invite>("GET", `/api/invites/${encodeURIComponent(token)}`),
  createFamily: (input: { family_name: string; member_name: string; relationship: string }) =>
    request<Me>("POST", "/api/families", input),
  joinFamily: (input: { token: string; member_name: string; relationship: string }) =>
    request<Me>("POST", "/api/families/join", input),
  switchMember: (memberId: string) => request<Me>("POST", "/api/session/switch", { member_id: memberId }),
};

/** A message that is safe to show the user for any thrown value. */
export function errorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : "Something went wrong. Please try again.";
}

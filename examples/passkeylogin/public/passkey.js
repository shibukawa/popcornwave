// Passkey ceremonies, driven from the page.
//
// The framework serves /auth/passkey/*; this file only converts between the
// Base64url the endpoints speak and the ArrayBuffers the WebAuthn API wants,
// which is the whole reason a script is needed at all.

const decode = (value) => {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
  return Uint8Array.from(binary, (c) => c.charCodeAt(0));
};

const encode = (buffer) =>
  btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");

async function post(path, body) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    // The endpoints are same-origin only, and a session cookie has to travel.
    credentials: "same-origin",
    body: JSON.stringify(body ?? {}),
  });
  if (!response.ok) {
    throw new Error("passkey: " + path + " failed with " + response.status);
  }
  return response.json();
}

// register adds a passkey to the account this browser is already signed in as.
// The endpoint refuses it unless that sign-in was recent, because adding a
// login method changes what the account can do.
export async function register() {
  const options = await post("/auth/passkey/register/begin");
  const credential = await navigator.credentials.create({
    publicKey: {
      ...options,
      challenge: decode(options.challenge),
      user: { ...options.user, id: decode(options.user.id) },
      excludeCredentials: (options.excludeCredentials ?? []).map((c) => ({
        ...c,
        id: decode(c.id),
      })),
    },
  });
  return post("/auth/passkey/register/finish", {
    id: credential.id,
    rawId: encode(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: encode(credential.response.clientDataJSON),
      attestationObject: encode(credential.response.attestationObject),
      transports: credential.response.getTransports?.() ?? [],
    },
  });
}

// login signs in with a passkey. No user name is asked for: the credential
// itself names the account.
export async function login() {
  const options = await post("/auth/passkey/login/begin");
  const assertion = await navigator.credentials.get({
    publicKey: {
      ...options,
      challenge: decode(options.challenge),
      allowCredentials: (options.allowCredentials ?? []).map((c) => ({
        ...c,
        id: decode(c.id),
      })),
    },
  });
  return post("/auth/passkey/login/finish", {
    id: assertion.id,
    rawId: encode(assertion.rawId),
    type: assertion.type,
    response: {
      clientDataJSON: encode(assertion.response.clientDataJSON),
      authenticatorData: encode(assertion.response.authenticatorData),
      signature: encode(assertion.response.signature),
      userHandle: assertion.response.userHandle
        ? encode(assertion.response.userHandle)
        : undefined,
    },
  });
}

// wire binds a control by id, so the page needs no inline script and the
// template stays free of JavaScript.
function wire(id, run) {
  const element = document.getElementById(id);
  if (!element) return;
  const event = element.tagName === "FORM" ? "submit" : "click";
  element.addEventListener(event, async (e) => {
    e.preventDefault();
    try {
      await run(element);
      location.reload();
    } catch (error) {
      const status = document.getElementById("passkey-status");
      if (status) status.textContent = String(error.message ?? error);
    }
  });
}

wire("passkey-login", login);
wire("passkey-register", register);

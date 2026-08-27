"use strict";

// This fixed native host returns only fresh-proved loopback proxy facts. It
// receives neither a Service Name nor a Target.
const nativeHost = "org.ardents.alpha_browser_entry";
const alphaURLs = ["http://*.ard/*", "https://*.ard/*"];
const proxyUsername = "ardents";
const answeredChallenges = new Set();

function failClosed() {
  // A loopback port that unprivileged Endpoint profiles never claim. Returning
  // a concrete proxy followed by null prevents this add-on from choosing DNS,
  // direct Internet, or a browser-configured proxy path when its native host
  // is unavailable.
  return [{ type: "http", host: "127.0.0.1", port: 1 }, null];
}

async function endpointAlphaProxy() {
  try {
    const response = await browser.runtime.sendNativeMessage(nativeHost, {
      operation: "loopback-proxy-port",
    });
    if (!Number.isInteger(response.port) || response.port < 1024 || response.port > 65535) {
      return failClosed();
    }
    // The terminal null is intentional: a connection failure at the selected
    // Endpoint proxy must not make Firefox try its ordinary proxy chain.
    return [{ type: "http", host: "127.0.0.1", port: response.port }, null];
  } catch (_) {
    return failClosed();
  }
}

async function answerAlphaProxyChallenge(details) {
  if (!details.isProxy || !details.challenger || details.challenger.host !== "127.0.0.1" ||
      !Number.isInteger(details.challenger.port) || answeredChallenges.has(details.requestId)) {
    return { cancel: true };
  }
  answeredChallenges.add(details.requestId);
  try {
    const response = await browser.runtime.sendNativeMessage(nativeHost, {
      operation: "loopback-proxy-authentication",
    });
    if (response.port !== details.challenger.port ||
        typeof response.password !== "string" || !/^[0-9a-f]{64}$/.test(response.password)) {
      return { cancel: true };
    }
    return { authCredentials: { username: proxyUsername, password: response.password } };
  } catch (_) {
    return { cancel: true };
  }
}

function forgetAlphaProxyChallenge(details) {
  answeredChallenges.delete(details.requestId);
}

browser.proxy.onRequest.addListener(endpointAlphaProxy, { urls: alphaURLs });
browser.webRequest.onAuthRequired.addListener(answerAlphaProxyChallenge, { urls: alphaURLs }, ["blocking"]);
browser.webRequest.onCompleted.addListener(forgetAlphaProxyChallenge, { urls: alphaURLs });
browser.webRequest.onErrorOccurred.addListener(forgetAlphaProxyChallenge, { urls: alphaURLs });

"use strict";

// The runner's temporary native host returns these two fields. This is a
// disposable protocol used to test the revalidation shape, not maintained code.
const nativeHost = "org.ardents.r117.proxy_auth";
const alphaURLs = ["http://*.ard/*", "https://*.ard/*"];

function validResponse(response) {
  return Number.isInteger(response.port) && response.port >= 1024 && response.port <= 65535;
}

async function native(operation) {
  const response = await browser.runtime.sendNativeMessage(nativeHost, { operation });
  if (!validResponse(response)) {
    throw new Error("native host returned an invalid proxy");
  }
  return response;
}

async function proxyForAlpha() {
  try {
    const response = await native("loopback-proxy-port");
    return [{ type: "http", host: "127.0.0.1", port: response.port }, null];
  } catch (_) {
    return [{ type: "http", host: "127.0.0.1", port: 1 }, null];
  }
}

async function provideLoopbackCredential(details) {
  if (!details.isProxy || details.challenger.host !== "127.0.0.1") {
    return { cancel: true };
  }
  try {
    // Re-read from the native host at the challenge, rather than sending a
    // value cached at the earlier routing decision to a recycled port.
    const response = await native("loopback-proxy-authentication");
    if (response.port !== details.challenger.port || typeof response.password !== "string" || response.password.length === 0) {
      return { cancel: true };
    }
    return { authCredentials: { username: "ardents", password: response.password } };
  } catch (_) {
    return { cancel: true };
  }
}

browser.proxy.onRequest.addListener(proxyForAlpha, { urls: alphaURLs });
browser.webRequest.onAuthRequired.addListener(provideLoopbackCredential, { urls: alphaURLs }, ["blocking"]);
browser.runtime.onInstalled.addListener(() => browser.tabs.create({ url: "http://reference.ard/" }));

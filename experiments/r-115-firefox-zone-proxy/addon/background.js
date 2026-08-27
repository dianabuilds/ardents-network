"use strict";

// The test launcher copies this fixture outside the repository and replaces
// 9 with its own ephemeral loopback listener port before web-ext loads it.
const loopbackPort = 9;
const zoneURLs = ["http://*.ard/*", "https://*.ard/*"];
const useNativeMessagePort = false;
const nativeHostName = "org.ardents.r115.fixture";
const temporaryProbeURL = "http://reference.ard/";
const temporaryUnavailableURL = "http://unavailable.ard/";
const temporaryHTTPSURL = "https://reference.ard/";
const temporaryOrdinaryURL = "http://ordinary.invalid/";

async function proxyOnlyArdZone() {
  // A trailing null is intentional: Firefox must not fall back to a user or
  // browser-defined proxy if loopback is absent. DNS/DoH/direct-path behavior
  // is a separate measurement and is not asserted by this fixture.
  let port = loopbackPort;
  if (useNativeMessagePort) {
    try {
      const response = await browser.runtime.sendNativeMessage(nativeHostName, {
        operation: "loopback-proxy-port",
      });
      if (!Number.isInteger(response.port) || response.port < 1024 || response.port > 65535) {
        return [null];
      }
      port = response.port;
    } catch (_) {
      return [null];
    }
  }
  return [{ type: "http", host: "127.0.0.1", port }, null];
}

browser.proxy.onRequest.addListener(proxyOnlyArdZone, { urls: zoneURLs });

// web-ext opens --start-url before it installs a temporary add-on. This probe
// therefore runs only after the listener above is registered. It is fixture
// instrumentation, not Browser Entry behavior: a real extension must open
// only a participant-requested authenticated name.
browser.runtime.onInstalled.addListener(async ({ temporary }) => {
  if (temporary) {
    await browser.tabs.create({ url: temporaryProbeURL });
    await new Promise((resolve) => setTimeout(resolve, 2500));
    await browser.tabs.create({ url: temporaryUnavailableURL });
    await new Promise((resolve) => setTimeout(resolve, 1000));
    await browser.tabs.create({ url: temporaryHTTPSURL });
    await new Promise((resolve) => setTimeout(resolve, 1000));
    await browser.tabs.create({ url: temporaryOrdinaryURL });
  }
});

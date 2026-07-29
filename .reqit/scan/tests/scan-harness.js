#!/usr/bin/env node
const baseURL = process.env.REQIT_SCAN_BASE_URL || process.env.BASE_URL;
const endpoints = [{"method":"GET","path":"/","sourceFiles":["flux/internal/mock/mock.go"],"lineNumbers":[112],"responseSchemas":{"200":{"additionalProperties":true,"type":"object"}}},{"method":"POST","path":"/payments","sourceFiles":["flux/internal/cli/cli_platform_test.go"],"lineNumbers":[16],"authSchemes":["bearer"],"requestSchema":{"additionalProperties":true,"type":"object"},"responseSchemas":{"200":{"additionalProperties":true,"type":"object"}}},{"method":"GET","path":"/status","sourceFiles":["flux/internal/cli/cli_platform_test.go","flux/internal/cli/cli_scan_test.go","flux/internal/workspace/repository_test.go"],"lineNumbers":[12,15,24],"authSchemes":["bearer"],"responseSchemas":{"200":{"additionalProperties":true,"type":"object"}}},{"method":"POST","path":"/users","sourceFiles":["flux/internal/scanner/scanner_test.go"],"lineNumbers":[17],"authSchemes":["bearer"],"requestSchema":{"properties":{"email":{"type":"string"},"password":{"type":"string"}},"required":["email","password"],"type":"object"},"responseSchemas":{"200":{"additionalProperties":true,"type":"object"}}},{"method":"GET","path":"/users/{id}","sourceFiles":["flux/internal/scanner/scanner_test.go"],"lineNumbers":[16],"authSchemes":["bearer"],"parameters":[{"name":"id","in":"path","required":true}],"responseSchemas":{"200":{"additionalProperties":true,"type":"object"}}}];

if (!baseURL) {
  console.error("REQIT_SCAN_BASE_URL (or BASE_URL) is required.");
  process.exit(1);
}

async function run() {
  let failed = 0;
  for (const ep of endpoints) {
    const url = new URL(ep.path, baseURL).toString();
    const method = ep.method || "GET";
    const body = (method === "GET" || method === "HEAD" || method === "OPTIONS") ? undefined : "{}";
    try {
      const resp = await fetch(url, {
        method,
        headers: { "content-type": "application/json" },
        body
      });
      if (resp.status >= 500) {
        failed++;
        console.error("FAIL", method, url, resp.status);
      } else {
        console.log("PASS", method, url, resp.status);
      }
    } catch (e) {
      failed++;
      console.error("FAIL", method, url, e.message);
    }
  }
  process.exit(failed === 0 ? 0 : 1);
}

run();

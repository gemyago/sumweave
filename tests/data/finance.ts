import { randomUUID } from 'node:crypto';

export const smokeTenant = {
  name: 'Playwright Smoke',
  displayCurrency: 'USD',
  seedDefaults: false,
};

export const smokeAccount = {
  name: 'Playwright Manual USD',
  kind: 'manual',
  currency: 'USD',
};

export function uniqueLabel(prefix: string): string {
  return `${prefix} ${randomUUID()}`;
}

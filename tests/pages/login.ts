import { expect, type Page } from '@playwright/test';
import type { Credentials } from '../clients/auth';

export class LoginPage {
  constructor(private readonly page: Page) {}

  async expectSignIn(): Promise<void> {
    await expect(this.page).toHaveURL(/#\/login$/);
    await expect(this.page.getByRole('heading', { name: 'Sign in', exact: true })).toBeVisible();
  }

  async signIn(credentials: Credentials): Promise<void> {
    await this.page.getByLabel('Username').fill(credentials.username);
    await this.page.getByLabel('Password').fill(credentials.password);
    await this.page.getByRole('button', { name: 'Sign in', exact: true }).click();
  }
}

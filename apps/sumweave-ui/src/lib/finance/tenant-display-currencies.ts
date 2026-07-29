export const supportedFinanceTenantDisplayCurrencies = ['USD', 'EUR', 'PLN', 'UAH'] as const

export type SupportedFinanceTenantDisplayCurrency = (typeof supportedFinanceTenantDisplayCurrencies)[number]

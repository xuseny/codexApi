declare module '@airwallex/components-sdk' {
  export interface AirwallexInitOptions {
    env?: 'demo' | 'prod' | string
    enabledElements?: string[]
    locale?: string
    [key: string]: unknown
  }

  export interface AirwallexCheckoutOptions {
    intent_id: string
    client_secret: string
    currency?: string
    country_code?: string
    successUrl?: string
    [key: string]: unknown
  }

  export interface AirwallexPayments {
    redirectToCheckout(options: AirwallexCheckoutOptions): string | void | Promise<string | void>
  }

  export interface AirwallexInitResult {
    payments?: AirwallexPayments
    [key: string]: unknown
  }

  export function init(options: AirwallexInitOptions): Promise<AirwallexInitResult>
}

export const BRAND = {
  productName: 'ExAPI',
  productTagline: 'Private AI API Gateway',
  githubRepo: 'immortal-autumn/sub2api',
  publicGatewayBasePath: '/v1',
} as const

export function getDefaultSiteName(): string {
  return BRAND.productName
}

export function getDefaultPaymentProductPrefix(): string {
  return BRAND.productName
}

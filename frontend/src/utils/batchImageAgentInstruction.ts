/** Marker used by the localized batch-image instruction templates. */
export const BATCH_IMAGE_AGENT_ENDPOINT_TOKEN = '__EXAPI_CONTROL_ENDPOINT__'
export const BATCH_IMAGE_AGENT_OPEN_BRACE_TOKEN = '__EXAPI_OPEN_BRACE__'
export const BATCH_IMAGE_AGENT_CLOSE_BRACE_TOKEN = '__EXAPI_CLOSE_BRACE__'

/**
 * Resolve the control-plane endpoint in a localized instruction template.
 *
 * The templates intentionally remain raw locale messages instead of using
 * vue-i18n interpolation: the instruction contains JSON braces which would
 * otherwise be parsed as message placeholders.
 */
export function resolveBatchImageAgentInstruction(template: unknown, endpoint: string): string {
  if (typeof template !== 'string') return ''
  return template
    .split(BATCH_IMAGE_AGENT_ENDPOINT_TOKEN).join(endpoint)
    .split(BATCH_IMAGE_AGENT_OPEN_BRACE_TOKEN).join('{')
    .split(BATCH_IMAGE_AGENT_CLOSE_BRACE_TOKEN).join('}')
}

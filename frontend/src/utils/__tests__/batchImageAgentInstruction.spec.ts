import { describe, expect, it } from 'vitest'

import enBatchImage from '@/i18n/locales/en/batchImage'
import zhBatchImage from '@/i18n/locales/zh/batchImage'
import {
  BATCH_IMAGE_AGENT_ENDPOINT_TOKEN,
  resolveBatchImageAgentInstruction,
} from '@/utils/batchImageAgentInstruction'

const endpoint = 'http://100.97.17.2:8027'

describe('batch-image Codex instruction localization', () => {
  it('uses English by default and resolves every control endpoint marker', () => {
    const instruction = resolveBatchImageAgentInstruction(enBatchImage.batchImage.agentInstruction, endpoint)

    expect(instruction).toContain('description: Use when the user wants to generate images in batches')
    expect(instruction).toContain(`Default endpoint:\n${endpoint}`)
    expect(instruction).toContain(`${endpoint}/api/v1/operator/batch-images/models?api_key_id=<api_key_id>`)
    expect(instruction).not.toContain(BATCH_IMAGE_AGENT_ENDPOINT_TOKEN)
    expect(instruction).not.toMatch(/[\u3400-\u9fff]/u)
  })

  it('keeps the Chinese instruction available when Chinese is selected', () => {
    const instruction = resolveBatchImageAgentInstruction(zhBatchImage.batchImage.agentInstruction, endpoint)

    expect(instruction).toContain('description: 当用户希望用 Gemini/Vertex 批量生成图片')
    expect(instruction).toContain(`默认端点：\n${endpoint}`)
    expect(instruction).toContain(`${endpoint}/api/v1/operator/batch-images/{id}/download?api_key_id=<api_key_id>`)
    expect(instruction).not.toContain(BATCH_IMAGE_AGENT_ENDPOINT_TOKEN)
  })

  it('fails closed for a missing or non-string locale message', () => {
    expect(resolveBatchImageAgentInstruction(undefined, endpoint)).toBe('')
    expect(resolveBatchImageAgentInstruction({ text: 'not a template' }, endpoint)).toBe('')
  })
})

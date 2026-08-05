import { beforeEach, describe, expect, it, vi } from 'vitest'

const { createWorkbook, toFile } = vi.hoisted(() => ({
  createWorkbook: vi.fn(),
  toFile: vi.fn(),
}))

vi.mock('write-excel-file/browser', () => ({
  default: createWorkbook,
}))

import { downloadExcelWorkbook } from '../excel'

describe('downloadExcelWorkbook', () => {
  beforeEach(() => {
    createWorkbook.mockReset()
    toFile.mockReset()
    createWorkbook.mockReturnValue({ toFile })
    toFile.mockResolvedValue(undefined)
  })

  it('writes the requested sheet and preserves string cells as strings', async () => {
    const rows = [
      ['Email', 'Tokens'],
      ['=HYPERLINK("https://example.test")', 42],
    ]

    await downloadExcelWorkbook(rows, 'usage.xlsx', 'Usage')

    expect(createWorkbook).toHaveBeenCalledWith(rows, {
      sheet: 'Usage',
      stickyRowsCount: 1,
    })
    expect(toFile).toHaveBeenCalledWith('usage.xlsx')
  })

  it('does not freeze a header for an empty sheet', async () => {
    await downloadExcelWorkbook([], 'empty.xlsx')

    expect(createWorkbook).toHaveBeenCalledWith([], {
      sheet: 'Sheet 1',
      stickyRowsCount: 0,
    })
  })
})

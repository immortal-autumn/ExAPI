export type ExcelCellValue = string | number | boolean | Date | null

export async function downloadExcelWorkbook(
  rows: ExcelCellValue[][],
  fileName: string,
  sheet = 'Sheet 1',
): Promise<void> {
  const { default: writeExcelFile } = await import('write-excel-file/browser')
  await writeExcelFile(rows, {
    sheet,
    stickyRowsCount: rows.length > 0 ? 1 : 0,
  }).toFile(fileName)
}

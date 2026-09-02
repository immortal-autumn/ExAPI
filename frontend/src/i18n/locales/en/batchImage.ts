export default {
  batchImage: {
    columns: {
      taskName: 'Task name',
      model: 'Model',
      apiKey: 'API key',
      result: 'Results',
      cost: 'Cost',
      downloadStatus: 'Download status',
    },
    status: {
      queued: 'Queued',
      running: 'Generating',
      processingResults: 'Processing results',
      settling: 'Settling',
      completed: 'Completed',
      failed: 'Failed',
      cancelled: 'Cancelled',
      outputDeleted: 'Results deleted',
      partialSuccess: 'Partially succeeded',
      allFailed: 'All failed',
    },
    itemStatus: {
      pending: 'Queued',
      succeeded: 'Succeeded',
      failed: 'Failed',
      cancelled: 'Cancelled',
      recovered: 'Recovered by retry',
    },
    filters: {
      searchTaskName: 'Search task name',
      allApiKeys: 'All API keys',
      allStatuses: 'All statuses',
      allDownloadStates: 'All download states',
      downloaded: 'Downloaded',
      notDownloaded: 'Not downloaded',
    },
    actions: {
      usageGuide: 'Usage guide',
      createJob: 'Create batch job',
      downloadSelected: 'Download selected',
      deleteRecords: 'Delete records',
      retryFailedItems: 'Retry failed items',
      cancelJob: 'Cancel job',
      downloadZip: 'Download ZIP',
      viewDetail: 'View details',
      download: 'Download',
      moreActions: 'More actions',
      copyInstruction: 'Copy instructions',
      submitJob: 'Submit job',
    },
    list: {
      selectedJobs: 'Selected {count} job | Selected {count} jobs',
      expandChildren: 'Expand {n} subtask | Expand {n} subtasks',
      collapseChildren: 'Collapse subtasks',
      childCount: '{n} subtask | {n} subtasks',
      childBadge: 'Subtask',
      keyNotRecorded: 'Not recorded',
      totalCount: 'of {n}',
      notDownloaded: 'Not downloaded',
      empty: 'No batch jobs yet',
      emptyHint: 'Use the button in the top-right corner to create a batch job.',
    },
    pagination: {
      pageNumber: 'Page {page}',
      pageItems: '{count} on this page',
    },
    promptPopover: {
      title: 'Full prompt',
      copied: 'Prompt copied',
    },
    detail: {
      title: 'Job details',
      aggregatedResult: 'Combined results',
      result: 'Results',
      cost: 'Cost',
      downloadStatus: 'Download status',
      items: 'Items',
      preview: 'Preview',
      previewZoom: 'Zoom compressed preview {id}',
      previewReload: 'Reload compressed preview',
      previewLoad: 'Load compressed preview',
      previewUnavailable: 'Preview unavailable',
      noImage: 'No image',
      loadingItems: 'Loading items...',
      noItems: 'No items yet',
      noItemsHint: 'Queued or generating jobs show submitted prompts first; image statuses update once results are processed.',
      mainTask: 'Main job: {name}',
      childTask: 'Subtask: {name}',
      holdCost: 'Hold {amount}',
    },
    itemResult: {
      recoveredByRetry: 'Previous failure recovered by a retry subtask',
      readyPreview: 'Image generated. Click to preview.',
      readyDownload: 'Image generated and ready to download.',
      noUsableImage: 'No usable image was generated.',
      cancelled: 'Job cancelled.',
      waiting: 'Waiting for results.',
      emptyImageOutput: 'The upstream returned a result, but this item has no image content. This usually means a single Gemini/Vertex generation failed or was blocked by safety policies.',
      providerItemFailed: 'The upstream result for this item has no usable image.',
    },
    imagePreview: {
      title: 'Image preview',
      notice: 'This is a compressed thumbnail cached locally in your browser, so quality is reduced. Download the ZIP to view the original image.',
    },
    create: {
      title: 'Create batch job',
      taskName: 'Task name',
      taskNamePlaceholder: 'Defaults to the current time if left empty',
      loadingKeys: 'Loading API keys...',
      selectKeyPlaceholder: 'Select a Gemini API key',
      noKeysHint: 'No Gemini API key is available for batch image generation. Create one and bind it to a Gemini group with batch image generation enabled first.',
      model: 'Model',
      imageSize: 'Image size',
      imageSizeHint: 'Batch jobs are currently submitted at a fixed 1K image size.',
      outputFormat: 'Output format',
      estimatedOutput: 'Estimated output',
      estimatedOutputValue: '{images} images / {prompts} prompts',
      promptAdded: '{count} added',
      promptPlaceholder: 'Paste a prompt, then add it to the list below',
      customIdPlaceholder: 'Custom ID (optional)',
      outputCountPerPrompt: 'Images per prompt',
      outputCountOption: '{n} image | {n} images',
      referenceImage: 'Reference images',
      removeReferenceImage: 'Remove reference image',
      limitsHint: 'Up to {maxPerItem} images per prompt and {maxPerJob} per job. The current model allows up to {refLimit} reference images per prompt; reference images consume input tokens once per generated image.',
      referenceCount: '{n} reference image | {n} reference images',
      noPrompts: 'No prompts added yet.',
      cancelNotice: 'Cancelling requests an upstream cancellation. Images already indexed as successful will still be billed, and the remaining hold will be released.',
      submittingNotice: 'Creating the upstream batch job. This usually takes a few seconds; please do not submit again.',
      modelNoReferenceImages: 'The current model does not support reference images.',
      refLimitReached: 'The current model allows up to {limit} reference images per prompt.',
      refLimitExceededIgnored: 'The current model allows up to {limit} reference images per prompt. Extra files were ignored.',
      refFormatUnsupported: 'Reference images must be PNG, JPEG, or WebP.',
      refFileTooLarge: '{name} exceeds 10MB and was ignored.',
    },
    guide: {
      title: 'Batch Image Generation Guide',
      uiTitle: 'How to use this page',
      step1: '1. Select a Gemini API key with batch image generation enabled. The model list shows the models available to that key’s group.',
      step2: '2. The task name can be left empty; the current time is used automatically on submit. Prompts are added to the list one by one, and each prompt can carry reference images and a repeat count.',
      step3: '3. After submitting, the job is queued first and the item list shows the submitted prompts. Image previews are not loaded by default; click the preview button on an item to load a single image.',
      step4: '4. Once completed you can download the ZIP. If some items failed, the More menu lets you retry only the failed items. Billing is still based on the number of successfully generated images; reference images are not billed separately.',
      skillTitle: 'Skill instructions for Codex',
      skillDesc: 'Tells Codex how to organize prompts, submit jobs, and download results on behalf of the user.',
    },
    agentInstruction: `---
name: sub2api-batch-image
description: Use when the user wants to generate images in batches with Gemini/Vertex, run prompts in bulk, download batch image results, or retry failed images.
---

You are the batch image execution agent in Codex. The user does not need to fill in the page form manually; organize the task name, prompt list, and output directory from the current chat, files, or context, and ask only when a key decision is missing.

Default endpoint:
__EXAPI_CONTROL_ENDPOINT__

You must:
1. Extract prompts from the user’s chat or attachments. Preserve each prompt in full and generate stable custom IDs in order, such as img_001 and img_002.
2. Infer the task name from the user’s request or context; if none is explicit, use the current time.
3. Infer the output directory from the user’s request or context; ask only when the user has not specified where to save the results.
4. Before submitting, calculate expected_output_count as the sum of output_count for every item. A single batch job is limited to 200 output images; split larger work into multiple jobs. Do not treat the reference-image attachment limit as the output-image limit.
5. If the user provides reference images, bind each image to the item where it is used. Reference images are inputs, not output images. Apply the model-specific per-item limits: Gemini 2.5 Flash Image allows up to 3 reference images per item, while Gemini 3 Pro Image allows up to 14. The backend also protects each expanded job with a total reference-image limit of 1000 and a decoded inline-base64 limit of 128 MB. These are rejection safeguards, not recommended batch sizes; split jobs proactively when there are many references or a large request body.
6. Reference images consume input tokens again for every output_count. For large jobs, repeated use of the same reference image, or large reference files, prefer gs:// file_uri or split the work into multiple jobs.
7. Choose the API key and model by first loading the currently available batch-image keys and models. If the user requested a model and the selected key supports it, use that model; otherwise use the default or first model available for that key. Do not show or ask about internal provider names.
8. Submit, poll, and download through the batch-image API; do not ask the user to fill in the page form manually.

API request rules:
- Access the control endpoint above through WireGuard and send X-ExAPI-Control-Request: 1 and Sec-Fetch-Site: same-origin. State-changing POST and DELETE requests must also send Origin: __EXAPI_CONTROL_ENDPOINT__ (with exactly the same origin as the request URL). Pass only the opaque api_key_id; never send Authorization or the raw gateway API key.
- Models: GET __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images/models?api_key_id=<api_key_id>
- Submit: POST __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images?api_key_id=<api_key_id>
- Status: GET __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images/{id}?api_key_id=<api_key_id>
- Items: GET __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images/{id}/items?api_key_id=<api_key_id>
- Download: GET __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images/{id}/download?api_key_id=<api_key_id>
- Cancel: POST __EXAPI_CONTROL_ENDPOINT__/api/v1/operator/batch-images/{id}/cancel?api_key_id=<api_key_id>

Request body:
{
  "model": "<a model available to the selected key>",
  "task_name": "<infer from chat; use the current time when empty>",
  "image_size": "1K",
  "response_mime_type": "image/png",
  "items": [
    {
      "custom_id": "img_001",
      "prompt": "<the first complete prompt>",
      "output_count": 1,
      "reference_images": [
        {
          "id": "face",
          "type": "subject",
          "mime_type": "image/png",
          "data": "<base64 without the data:image/png;base64, prefix>"
        }
      ]
    }
  ]
}

Requirements:
- Never write an API key to the repository, logs, commit history, or final response.
- Never write reference-image base64 data to the final response, logs, or public files. A recovery record may contain only reference filenames, purposes, counts, and the request JSON path; if the request JSON contains base64, keep it in the user-specified output directory and do not commit it.
- output_count means repeating the same prompt and reference images, defaults to 1, and is limited to 4 per item. The system expands this into real task items; it does not depend on a single Gemini request returning multiple images. Confirm that expected_output_count is at most 200 before submitting and split larger work into multiple jobs.
- Batch-image billing is currently based on successful output-image count, with no separate reference-image charge. Explain that references still add some upstream input-token and temporary-storage cost, repeated for each output_count; the displayed hold and settlement amount is based on output-image count.
- Immediately after a successful submission, write a local recovery record such as batch-image-resume.json in the output directory. Never store an API key in the recovery record.
- The recovery record must include endpoint, task_name, batch_id, model, output_dir, request_file, submitted_at, last_status, status_url, items_url, download_url, prompt_count, expected_output_count, and either a custom-ID-to-prompt map or a request JSON path for retrying failures.
- Update the recovery record after every status query with last_checked_at, last_status, success count, failure count, actual cost, and a failure summary. A later session must be able to query, download, or retry from this record after interruption.
- Do not poll frequently. Wait about 20 to 30 seconds before the first query; query queued jobs every 60 to 120 seconds. If a job remains queued for 3 consecutive queries, stop active polling, tell the user it is still queued, keep the recovery record, and continue other work or wait for the user to request a resume.
- Poll running jobs about every 60 seconds, or less often under high server load or for large jobs. Poll processing_results and other near-completion states every 20 to 45 seconds.
- When a job completes, report its task name, ID, success count, failure count, actual cost, and save path.
- Download successful images only. For partial failures, first show the failed custom ID, error code, source, and a brief reason.
- Retry failed items only; never resubmit successful items. If a historical job did not save failed-item prompts, explain that automatic retry is unavailable and ask the user for the original prompts.
- Before cancelling, warn that images already indexed as successful will still be billed and the remaining hold will be released.
- Load image previews on demand; do not automatically load image content for every list item.
`,
    messages: {
      loadKeysFailed: 'Failed to load API keys.',
      loadModelsFailed: 'Failed to load available models.',
      loadJobsFailed: 'Failed to load batch jobs.',
      selectApiKey: 'Select an available Gemini API key.',
      noModelsForKey: 'This key has no available batch image models.',
      selectModel: 'Select a model.',
      promptRequired: 'Enter at least one prompt.',
      submitted: 'Batch job submitted.',
      submitFailed: 'Failed to submit the batch job.',
      refreshFailed: 'Failed to refresh the job.',
      cancelConfirm: 'Cancellation will be sent upstream. Images already indexed as successful will still be billed, and the remaining hold will be released. Continue?',
      cancelled: 'Cancellation requested.',
      cancelFailed: 'Failed to cancel the job.',
      batchDownloadStarted: 'Downloads for the selected jobs have started.',
      downloadFailed: 'Failed to download the result.',
      retrySubmitted: 'Retry job submitted for failed items.',
      retryFailed: 'Failed to retry failed items.',
      retryMissingPrompts: 'This job does not have saved prompts for failed items, so it cannot be retried automatically. Recreate it with the original prompt.',
      retryTaskNameSuffix: 'Retry failed items',
      deleteConfirm: 'This hides the job from your list while keeping billing records. Delete it?',
      deleteSelectedConfirm: 'This hides the selected jobs from your list while keeping billing records. Delete them?',
      deleted: 'Job record deleted.',
      deleteFailed: 'Failed to delete the job record.',
      loadItemsFailed: 'Failed to load item details.',
      loadPreviewFailed: 'Failed to load the image preview.',
      copiedInstruction: 'Batch image instructions copied.',
      loadingModels: 'Loading available models...',
      noModels: 'No available models',
      noModelsHint: 'This key’s group has no models configured for batch image generation.',
      noCompatibleAccount: 'No usable upstream batch image account is available for this key’s group. Contact an administrator to check the group’s schedulable Gemini API key or Vertex service account and model support.',
      unsupportedProvider: 'The batch image provider for this job is not available. Contact an administrator to check the batch image provider configuration.',
      providerSubmitFailed: 'The upstream batch image job failed to submit. Contact an administrator to check the upstream account, model permission, or provider status.',
      vertexGcsBucketMissing: 'Vertex batch image generation is missing the managed GCS bucket configuration. Contact an administrator to configure BATCH_IMAGE_VERTEX_MANAGED_GCS_BUCKET before submitting again.',
      queueFailed: 'The task queue is temporarily unavailable, so the batch job was not queued. Contact an administrator to check the queue service.',
      billingHoldFailed: 'The cost hold failed, so the batch job was not submitted. Contact an administrator to check billing or balance hold service.',
      groupDisabled: 'Batch image generation is not enabled for this key’s group. Choose another enabled key or contact an administrator.',
      pricingMissing: 'The selected model does not have batch image pricing configured. Contact an administrator to add pricing first.',
      insufficientBalance: 'Insufficient balance to hold the estimated batch image cost.',
      invalidModel: 'Select a batch image model available for the current key.',
      invalidItems: 'The prompt list is invalid. Check that it is not empty, within the item limit, and still using 1K image size.',
      duplicateCustomId: 'Custom IDs in the prompt list must be unique.',
      promptTooLong: 'One prompt is too long. Shorten it and try again.',
      invalidReferenceImage: 'A reference image is invalid. Use PNG, JPEG, or WebP under 10 MB.',
      tooManyReferenceImages: 'Too many reference images. Flash Image allows up to 3 per item, Pro Image allows up to 14, and each job allows up to 1000 total.',
      referenceImagesTooLarge: 'Reference images are too large. Inline reference images are limited to 128 MB per job; use gs:// file_uri or split the job for large batches.',
      tooManyOutputImages: 'Too many expected output images. Each prompt can request up to 4 images, and each job can generate up to 200 images.',
      idempotencyConflict: 'This submission conflicts with a previous request ID. Refresh the page and submit again.',
      notReady: 'The job is not complete yet. Download will be available after completion.',
      outputDeleted: 'The result files for this job have already been cleaned up.',
      resultMissing: 'The result file is unavailable. It may have been cleaned up, storage permissions may be broken, or storage settings may have changed. Contact an administrator to check the result file.',
      itemFailed: 'This item has no successful image to preview.',
      itemImageIndexOutOfRange: 'This item has no previewable image.',
      downloadLimited: 'Too many download requests are active. Please try again later.',
      downloadTooLarge: 'This ZIP is too large for a single download. Download fewer items at once or ask an administrator to raise the batch download limit.',
      deleteNotReady: 'Job records can only be deleted after the job finishes.',
      disabled: 'Batch image generation is currently disabled.',
      authRequired: 'The current API key is unavailable or expired. Select the key again.',
      adminReference: 'Send the error code and request ID to an administrator for troubleshooting.',
      errorReference: 'Error detail',
      errorCodeRef: 'code: {code}',
      requestIdRef: 'request ID: {id}',
      httpStatusRef: 'HTTP status: {status}',
    },
  },
}

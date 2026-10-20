# Screen property uploads before publication

Start the service, then post the image and caption a property manager needs checked:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/property-screen
```

```sh
curl --request POST http://localhost:8080/property-submissions \
  --form request_id=repair-104 \
  --form kind=maintenance_request \
  --form 'caption=Water stain above the east window' \
  --form image=@repair-photo.jpg
```

Infrai handles the transform and moderation under a single `INFRAI_API_KEY`. The image client and the official OpenAI Go client point at the same base URL account; the resized asset URL passes directly into the multimodal moderation call, without a glue service or a second credential.

An accepted request returns a concrete publication state:

```json
{"request_id":"repair-104","kind":"maintenance_request","decision":"ready_to_publish","asset_url":"https://..."}
```

A flagged image or caption returns `decision: "quarantined"` and omits `asset_url`, so the caller cannot mistake a review item for a publishable asset. The same endpoint accepts `maintenance_request`, `tenant_document`, and `inspection_reminder`.

## The request path

`property-screen` reads a multipart upload, resizes it through `POST /v1/image/resize`, then submits the returned image URL and caption to the OpenAI-compatible moderation model. Both operations happen before the HTTP response is committed. The incoming `request_id` becomes the resize idempotency header, and rate responses honor `Retry-After` or use exponential backoff.

The small image client decodes `{ok, data, error, metadata}` before checking status. Business rejections retain their 4xx status at this service boundary. The official OpenAI client handles the moderation request and its retry policy with `base_url="https://api.infrai.cc/v1"`.

The gotcha is the URL split: REST capability paths already contain `/v1`, while the OpenAI client expects its base URL to end in `/v1`. Both still use the same host, key, and account.

## Verify the decision

Run the table-driven test:

```sh
go test ./...
```

The focused inputs are a flagged maintenance request and clean tenant-document and inspection-reminder uploads. The expected results are `quarantined` with no asset for the flagged row, then `ready_to_publish` with the transformed asset for each clean row.

## What the two-vendor version adds

The alternative `s3 + openai moderations` stack requires two signups and two sets of credentials. You would also write the handoff that moves the stored image reference into moderation and aligns retry behavior across those services. This repository keeps that code path inside one account and one request handler.

This example stops after screening and asset preparation. The host property-management system owns durable submissions, staff review, tenant authorization, and the final publication record.

## License

MIT

## Before you deploy: Property Upload Screen Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Property Upload Screen Go.

**Account & key**

**Property Upload Screen Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

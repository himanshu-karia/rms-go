# Project DNA API (draft OpenAPI snippets)

Version: draft-2026-01-02

```yaml
openapi: 3.0.3
info:
  title: Project DNA API
  version: draft-2026-01-02
paths:
  /api/project-dna/{projectId}/sensors:
    get:
      summary: List sensors for a project
      parameters:
        - name: projectId
          in: path
          required: true
          schema: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  sensors:
                    type: array
                    items:
                      $ref: '#/components/schemas/Sensor'
    put:
      summary: Bulk upsert sensors for a project
      parameters:
        - name: projectId
          in: path
          required: true
          schema: { type: string }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                sensors:
                  type: array
                  items:
                    $ref: '#/components/schemas/Sensor'
      responses:
        '200': { description: Upserted }

  /api/project-dna/{projectId}/thresholds:
    get:
      summary: Get thresholds (merged project + optional device overrides)
      parameters:
        - name: projectId
          in: path
          required: true
          schema: { type: string }
        - name: device
          in: query
          required: false
          schema: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  thresholds:
                    type: array
                    items:
                      $ref: '#/components/schemas/Threshold'
                  source:
                    type: string
                    description: one of default|override-merged
    put:
      summary: Upsert project-level thresholds
      parameters:
        - name: projectId
          in: path
          required: true
          schema: { type: string }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                thresholds:
                  type: array
                  items:
                    $ref: '#/components/schemas/Threshold'
      responses:
        '200': { description: Upserted }

  /api/project-dna/{projectId}/thresholds/{deviceId}:
    put:
      summary: Upsert device-level threshold overrides
      parameters:
        - name: projectId
          in: path
          required: true
          schema: { type: string }
        - name: deviceId
          in: path
          required: true
          schema: { type: string }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                thresholds:
                  type: array
                  items:
                    $ref: '#/components/schemas/Threshold'
      responses:
        '200': { description: Upserted }

components:
  schemas:
    Sensor:
      type: object
      required: [param, label, required]
      properties:
        param: { type: string, description: canonical identifier }
        label: { type: string }
        unit: { type: string, nullable: true }
        min: { type: number, nullable: true }
        max: { type: number, nullable: true }
        resolution: { type: number, nullable: true }
        required: { type: boolean }
        notes: { type: string, nullable: true }
        topic_template: { type: string, nullable: true }
        updated_at: { type: string, format: date-time }
    Threshold:
      type: object
      required: [param]
      properties:
        param: { type: string }
        warn_low: { type: number, nullable: true }
        warn_high: { type: number, nullable: true }
        alert_low: { type: number, nullable: true }
        alert_high: { type: number, nullable: true }
        origin: { type: string, description: protocol-default|override-api|firmware, nullable: true }
        updated_at: { type: string, format: date-time }
```

Notes:
- Merge logic for `GET /thresholds` should overlay device overrides on project defaults server-side.
- Cache keys: `config:project:{projectId}` for sensors, `config:thresholds:{projectId}` for defaults, `config:thresholds:{projectId}:{deviceId}` for overrides.
- `param` remains the single identifier; rules/transforms/strict allow-list must use it consistently.

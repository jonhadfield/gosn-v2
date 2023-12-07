package docs

var NoteContentFull = `{
  "type": "object",
  "properties": {
    "text": {
      "type": "string"
    },
    "title": {
      "type": "string"
    },
    "noteType": {
      "type": "string"
    },
    "references": {
      "type": "array",
      "items": {
        "type": "object",
        "uniqueItems": true
      }
    },
    "appData": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "additionalProperties": true
      }
    },
    "editorIdentifier": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9_\\-.]*$"
    },
    "spellcheck": {
      "type": "boolean"
    }
  },
  "required": [
    "title",
    "text",
    "references",
    "preview_plain",
    "noteType",
    "editorIdentifier",
    "spellcheck",
    "appData"
  ]
}
`

var NoteContentMinimal = `{
  "type": "object",
  "properties": {
    "text": {
      "type": "string"
    },
    "title": {
      "type": "string"
    },
    "noteType": {
      "type": "string"
    },
    "references": {
      "type": "array",
      "items": {
        "type": "object",
        "uniqueItems": true
      }
    },
    "appData": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "additionalProperties": true
      }
    },
    "editorIdentifier": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9_\\-.]*$"
    },
    "spellcheck": {
      "type": "boolean"
    }
  },
  "required": [
    "title",
    "text",
    "references",
    "appData"
  ]
}
`

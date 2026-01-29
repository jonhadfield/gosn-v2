# Standard Notes Item Type Coverage Analysis - gosn-v2 vs Official App

## Executive Summary

This analysis compares the item type implementations in gosn-v2 with the official Standard Notes app to identify missing attributes and ensure complete coverage. The analysis is based on the official Standard Notes app repository code at `/Users/hadfielj/Repositories/app`.

## Item Type Coverage Comparison

### Base ItemContent Interface

**Official Standard Notes (ItemContent)**:
```typescript
export interface ItemContent {
  references: ContentReference[]
  conflict_of?: string
  protected?: boolean
  trashed?: boolean
  pinned?: boolean
  archived?: boolean
  starred?: boolean
  locked?: boolean
  appData?: AppData
}
```

**gosn-v2 Implementation**:
```go
type AppDataContent struct {
  OrgStandardNotesSN           OrgStandardNotesSNDetail           `json:"org.standardnotes.sn"`
  OrgStandardNotesSNComponents OrgStandardNotesSNComponentsDetail `json:"org.standardnotes.sn.components,omitempty"`
}

// Base attributes are in ItemCommon (UUID, ContentType, CreatedAt, etc.)
```

**❌ Missing Base Attributes in gosn-v2**:
- `conflict_of` (conflict resolution)
- `protected` (protection status)
- `trashed` (trash status) - Note: gosn-v2 has `Trashed *bool` in NoteContent only
- `pinned` (pin status)
- `archived` (archive status)
- `starred` (star status)
- `locked` (lock status)

### 1. Note Content

**Official Standard Notes (NoteContentSpecialized)**:
```typescript
export interface NoteContentSpecialized {
  title: string
  text: string
  hidePreview?: boolean
  preview_plain?: string
  preview_html?: string
  spellcheck?: boolean
  editorWidth?: EditorLineWidth
  noteType?: NoteType
  editorIdentifier?: string
  authorizedForListed?: boolean
}
```

**gosn-v2 Implementation**:
```go
type NoteContent struct {
  Title            string             `json:"title"`
  Text             string             `json:"text"`
  ItemReferences   ItemReferences     `json:"references"`
  AppData          NoteAppDataContent `json:"appData"`
  PreviewPlain     string             `json:"preview_plain"`
  Spellcheck       bool               `json:"spellcheck"`
  PreviewHtml      string             `json:"preview_html"`
  NoteType         string             `json:"noteType"`
  EditorIdentifier string             `json:"editorIdentifier"`
  Trashed          *bool              `json:"trashed,omitempty"`
  HidePreview      bool               `json:"hidePreview,omitempty"`
}
```

**❌ Missing Note Attributes**:
- `editorWidth` (editor line width settings)
- `authorizedForListed` (listed.to authorization)

**✅ gosn-v2 Has Extra**:
- Advanced checklist support (`ToAdvancedCheckList()`)
- `Trashed` as note-specific attribute

### 2. Tag Content

**Official Standard Notes (TagContentSpecialized)**:
```typescript
export interface TagContentSpecialized {
  title: string
  expanded: boolean
  iconString: IconType | EmojiString
  preferences?: TagPreferences
}
```

**gosn-v2 Implementation**:
```go
type TagContent struct {
  Title          string         `json:"title"`
  ItemReferences ItemReferences `json:"references"`
  AppData        AppDataContent `json:"appData"`
  IconString     string         `json:"iconString,omitempty"`
  Expanded       bool           `json:"expanded,omitempty"`
  ParentId       string         `json:"parentId,omitempty"`
}
```

**❌ Missing Tag Attributes**:
- `preferences` (TagPreferences object)

**✅ gosn-v2 Has Extra**:
- `ParentId` (for nested tags)

### 3. Component Content

**Official Standard Notes (ComponentContentSpecialized)**:
```typescript
export type ComponentContentSpecialized = {
  disassociatedItemIds?: string[]
  associatedItemIds?: string[]
  local_url?: string
  hosted_url?: string
  offlineOnly?: boolean
  name: string
  autoupdateDisabled?: boolean
  package_info: ComponentPackageInfo
  area: ComponentArea
  permissions?: ComponentPermission[]
  valid_until: Date | number
  legacy_url?: string
  isDeprecated?: boolean
  active?: boolean        // @deprecated
  url?: string           // @deprecated
  componentData?: Record<string, unknown>  // @deprecated
}
```

**gosn-v2 Implementation**:
```go
type ComponentContent struct {
  Identifier         string         `json:"identifier"`
  LegacyURL          string         `json:"legacy_url,omitempty"`
  HostedURL          string         `json:"hosted_url,omitempty"`
  LocalURL           string         `json:"local_url,omitempty"`
  URL                string         `json:"url,omitempty"`
  ValidUntil         string         `json:"valid_until,omitempty"`
  OfflineOnly        FlexibleBool   `json:"offlineOnly,omitempty"`
  Name               string         `json:"name"`
  Area               string         `json:"area"`
  PackageInfo        interface{}    `json:"package_info,omitempty"`
  Permissions        []interface{}  `json:"permissions,omitempty"`
  Active             interface{}    `json:"active,omitempty"`
  AutoUpdateDisabled FlexibleBool   `json:"autoupdateDisabled,omitempty"`
  ComponentData      interface{}    `json:"componentData,omitempty"`
  DissociatedItemIds []string       `json:"disassociatedItemIds,omitempty"`
  AssociatedItemIds  []string       `json:"associatedItemIds,omitempty"`
  ItemReferences     ItemReferences `json:"references"`
  AppData            AppDataContent `json:"appData"`
}
```

**❌ Missing Component Attributes**:
- `isDeprecated` (deprecation flag)

**✅ gosn-v2 Has Extra**:
- `Identifier` (component identifier)
- Strong typing with `FlexibleBool` for JSON compatibility

### 4. SmartView Content (extends TagContent)

**Official Standard Notes (SmartViewContent)**:
```typescript
export interface SmartViewContent extends TagContent {
  predicate: PredicateJsonForm
}
```

**gosn-v2 Implementation**:
gosn-v2 has `SNItemTypeSmartTag` but no dedicated SmartView content structure with predicate support.

**❌ Missing SmartView Attributes**:
- `predicate` (PredicateJsonForm for smart filtering)

### 5. File Content

**Official Standard Notes (FileContentSpecialized)**:
```typescript
export type FileContentSpecialized = {
  remoteIdentifier: string
  name: string
  key: string
  encryptionHeader: string
  mimeType: string
  decryptedSize: DecryptedBytesLength
  encryptedChunkSizes: EncryptedBytesLength[]
  // Deprecated fields for backward compatibility
  size?: DecryptedBytesLength
  chunkSizes?: EncryptedBytesLength[]
} & FileMetadata
```

**gosn-v2 Implementation**:
gosn-v2 has `SNItemTypeFile` but no detailed File content structure implemented.

**❌ Missing File Attributes**:
- `remoteIdentifier` (server file identifier)
- `name` (filename)
- `key` (encryption key)
- `encryptionHeader` (encryption metadata)
- `mimeType` (file content type)
- `decryptedSize` (actual file size)
- `encryptedChunkSizes` (encrypted chunk sizes array)

### 6. TrustedContact Content

**Official Standard Notes (TrustedContactContentSpecialized)**:
```typescript
export type TrustedContactContentSpecialized = {
  name: string
  contactUuid: string
  publicKeySet: ContactPublicKeySetJsonInterface
  isMe: boolean
}
```

**gosn-v2 Implementation**:
❌ **Not implemented** - No TrustedContact support in gosn-v2

**❌ Missing TrustedContact Type & Attributes**:
- Complete `TrustedContact` item type missing
- `name` (contact display name)
- `contactUuid` (unique contact identifier)
- `publicKeySet` (public key set for encryption)
- `isMe` (self-contact flag)

### 7. VaultListing Content

**Official Standard Notes (VaultListingContentSpecialized)**:
```typescript
export interface VaultListingContentSpecialized extends SpecializedContent {
  systemIdentifier: KeySystemIdentifier
  rootKeyParams: KeySystemRootKeyParamsInterface
  keyStorageMode: KeySystemRootKeyStorageMode
  name: string
  description?: string
  iconString: IconType | EmojiString
  sharing?: VaultListingSharingInfo
}
```

**gosn-v2 Implementation**:
❌ **Not implemented** - No VaultListing support in gosn-v2

**❌ Missing VaultListing Type & Attributes**:
- Complete `VaultListing` item type missing
- `systemIdentifier` (key system ID)
- `rootKeyParams` (key parameters)
- `keyStorageMode` (storage mode for keys)
- `name` (vault name)
- `description` (vault description)
- `iconString` (vault icon)
- `sharing` (sharing configuration)

### 8. KeySystemRootKey Content

**Official Standard Notes (KeySystemRootKeyContentSpecialized)**:
```typescript
export type KeySystemRootKeyContentSpecialized = {
  keyParams: KeySystemRootKeyParamsInterface
  systemIdentifier: KeySystemIdentifier
  key: string
  keyVersion: ProtocolVersion
  token: string
}
```

**gosn-v2 Implementation**:
❌ **Not implemented** - No KeySystemRootKey support in gosn-v2

**❌ Missing KeySystemRootKey Type & Attributes**:
- Complete `KeySystemRootKey` item type missing
- `keyParams` (key derivation parameters)
- `systemIdentifier` (system identifier)
- `key` (encrypted root key)
- `keyVersion` (protocol version)
- `token` (authentication token)

### 9. KeySystemItemsKey Content

**Official Standard Notes (KeySystemItemsKeyContentSpecialized)**:
```typescript
export interface KeySystemItemsKeyContentSpecialized extends SpecializedContent {
  version: ProtocolVersion
  creationTimestamp: number
  itemsKey: string
  rootKeyToken: string
}
```

**gosn-v2 Implementation**:
❌ **Not implemented** - No KeySystemItemsKey support in gosn-v2

**❌ Missing KeySystemItemsKey Type & Attributes**:
- Complete `KeySystemItemsKey` item type missing
- `version` (protocol version)
- `creationTimestamp` (creation time)
- `itemsKey` (encrypted items key)
- `rootKeyToken` (root key reference token)

### 10. UserPreferences Content

**Official Standard Notes (SNUserPrefs)**:
```typescript
export class SNUserPrefs extends DecryptedItem {
  // Uses key-value preference system via getAppDomainValue()
  getPref<K extends PrefKey>(key: K): PrefValue[K] | undefined
}
```

**gosn-v2 Implementation**:
```go
// Has SNItemTypeUserPreferences constant but no detailed implementation
```

**❌ Missing UserPreferences Attributes**:
- Structured preference key-value system
- Typed preference access methods
- `getPref` functionality for type-safe preference access

## Content Types Coverage

### Official Standard Notes Content Types:
1. ✅ `ContentType.TYPES.Note` - **Implemented** (gosn-v2: `SNItemTypeNote`)
2. ✅ `ContentType.TYPES.Tag` - **Implemented** (gosn-v2: `SNItemTypeTag`)
3. ✅ `ContentType.TYPES.Component` - **Implemented** (gosn-v2: `SNItemTypeComponent`)
4. ✅ `ContentType.TYPES.Theme` - **Implemented** (gosn-v2: `SNItemTypeTheme`)
5. ⚠️ `ContentType.TYPES.File` - **Partial** (gosn-v2: `SNItemTypeFile` - missing content structure)
6. ⚠️ `ContentType.TYPES.SmartView` - **Partial** (gosn-v2: `SNItemTypeSmartTag` - missing predicate)
7. ⚠️ `ContentType.TYPES.UserPrefs` - **Partial** (gosn-v2: `SNItemTypeUserPreferences` - missing structure)
8. ✅ `ContentType.TYPES.ActionsExtension` - **Implemented** (gosn-v2: `SNItemTypeExtension`)
9. ✅ `ContentType.TYPES.ExtensionRepo` - **Implemented** (gosn-v2: `SNItemTypeExtensionRepo`)
10. ❌ `ContentType.TYPES.TrustedContact` - **Missing** (collaboration feature)
11. ❌ `ContentType.TYPES.VaultListing` - **Missing** (shared vault feature)
12. ❌ `ContentType.TYPES.KeySystemRootKey` - **Missing** (advanced encryption)
13. ❌ `ContentType.TYPES.KeySystemItemsKey` - **Missing** (advanced encryption)
14. ✅ `ContentType.TYPES.ItemsKey` - **Implemented** (gosn-v2: `SNItemTypeItemsKey`)

### gosn-v2 Content Types:
```go
const (
  SNItemTypeNote                 = "Note"
  SNItemTypeTag                  = "Tag"
  SNItemTypeComponent            = "SN|Component"
  SNItemTypeItemsKey             = "SN|ItemsKey"
  SNItemTypeTheme                = "SN|Theme"
  SNItemTypePrivileges           = "SN|Privileges"
  SNItemTypeExtension            = "Extension"
  SNItemTypeSFExtension          = "SF|Extension"
  SNItemTypeSFMFA                = "SF|MFA"
  SNItemTypeSmartTag             = "SN|SmartTag"
  SNItemTypeFileSafeFileMetaData = "SN|FileSafe|FileMetadata"
  SNItemTypeFileSafeIntegration  = "SN|FileSafe|Integration"
  SNItemTypeFileSafeCredentials  = "SN|FileSafe|Credentials"
  SNItemTypeUserPreferences      = "SN|UserPreferences"
  SNItemTypeExtensionRepo        = "SN|ExtensionRepo"
  SNItemTypeFile                 = "SN|File"
)
```

## Key Findings

### ✅ Strong Coverage Areas:
1. **Core Item Types**: Note, Tag, Component are well implemented
2. **Legacy Support**: FileSafe items, SF extensions covered
3. **Advanced Features**: FlexibleBool for JSON compatibility
4. **Extension System**: Components, themes, extensions supported

### ❌ Missing Critical Features:

#### 1. Base ItemContent Attributes (High Priority):
- `conflict_of` - Important for conflict resolution
- `protected`, `pinned`, `archived`, `starred`, `locked` - Core user features
- `trashed` - Should be available for all item types, not just notes

#### 2. Missing Item Types (High Priority):
- `TrustedContact` - Complete item type for collaboration and contact management
- `VaultListing` - Complete item type for shared vault functionality
- `KeySystemRootKey` - Complete item type for advanced key system encryption
- `KeySystemItemsKey` - Complete item type for key system items encryption

#### 3. Incomplete Item Content Structures (Medium Priority):
- `FileContent` - Has type constant but missing all file-specific attributes
- `SmartViewContent` - Missing `predicate` field for smart filtering
- `UserPrefsContent` - Missing structured preference system

#### 4. Enhanced Content Features (Low Priority):
- `editorWidth` in NoteContent
- `authorizedForListed` in NoteContent
- `preferences` in TagContent
- `isDeprecated` in ComponentContent

### 🔄 Type Inconsistencies:
1. **ValidUntil**: Official uses `Date | number`, gosn-v2 uses `string`
2. **Permissions**: Official uses typed `ComponentPermission[]`, gosn-v2 uses `[]interface{}`
3. **PackageInfo**: Official uses typed `ComponentPackageInfo`, gosn-v2 uses `interface{}`

## Recommendations

### High Priority (Essential for Full Compatibility):

1. **Add Base ItemContent Attributes**:
```go
type ItemCommon struct {
  // ... existing fields ...
  ConflictOf *string `json:"conflict_of,omitempty"`
  Protected  bool    `json:"protected,omitempty"`
  Trashed    bool    `json:"trashed,omitempty"`
  Pinned     bool    `json:"pinned,omitempty"`
  Archived   bool    `json:"archived,omitempty"`
  Starred    bool    `json:"starred,omitempty"`
  Locked     bool    `json:"locked,omitempty"`
}
```

2. **Implement Missing Item Types**:

```go
// TrustedContact
type TrustedContactContent struct {
  Name         string                 `json:"name"`
  ContactUUID  string                 `json:"contactUuid"`
  PublicKeySet interface{}            `json:"publicKeySet"` // ContactPublicKeySetJsonInterface
  IsMe         bool                   `json:"isMe"`
  ItemReferences ItemReferences       `json:"references"`
  AppData      AppDataContent         `json:"appData"`
}

// VaultListing
type VaultListingContent struct {
  SystemIdentifier string             `json:"systemIdentifier"`
  RootKeyParams    interface{}        `json:"rootKeyParams"`
  KeyStorageMode   string             `json:"keyStorageMode"`
  Name             string             `json:"name"`
  Description      string             `json:"description,omitempty"`
  IconString       string             `json:"iconString"`
  Sharing          interface{}        `json:"sharing,omitempty"`
  ItemReferences   ItemReferences     `json:"references"`
  AppData          AppDataContent     `json:"appData"`
}

// KeySystemRootKey
type KeySystemRootKeyContent struct {
  KeyParams        interface{}        `json:"keyParams"`
  SystemIdentifier string             `json:"systemIdentifier"`
  Key              string             `json:"key"`
  KeyVersion       string             `json:"keyVersion"`
  Token            string             `json:"token"`
  ItemReferences   ItemReferences     `json:"references"`
  AppData          AppDataContent     `json:"appData"`
}

// KeySystemItemsKey
type KeySystemItemsKeyContent struct {
  Version           string             `json:"version"`
  CreationTimestamp int64              `json:"creationTimestamp"`
  ItemsKey          string             `json:"itemsKey"`
  RootKeyToken      string             `json:"rootKeyToken"`
  ItemReferences    ItemReferences     `json:"references"`
  AppData           AppDataContent     `json:"appData"`
}
```

3. **Complete Existing Item Content Structures**:

```go
// File Content
type FileContent struct {
  RemoteIdentifier     string         `json:"remoteIdentifier"`
  Name                 string         `json:"name"`
  Key                  string         `json:"key"`
  EncryptionHeader     string         `json:"encryptionHeader"`
  MimeType             string         `json:"mimeType"`
  DecryptedSize        int64          `json:"decryptedSize"`
  EncryptedChunkSizes  []int64        `json:"encryptedChunkSizes"`
  // Deprecated fields for backward compatibility
  Size                 *int64         `json:"size,omitempty"`
  ChunkSizes           []int64        `json:"chunkSizes,omitempty"`
  ItemReferences       ItemReferences `json:"references"`
  AppData              AppDataContent `json:"appData"`
}

// SmartView Content (extends TagContent)
type SmartViewContent struct {
  TagContent                           // Embed TagContent
  Predicate    interface{}             `json:"predicate"` // PredicateJsonForm
}

// Enhanced UserPreferences
type UserPreferencesContent struct {
  Preferences    map[string]interface{} `json:"preferences"`
  ItemReferences ItemReferences         `json:"references"`
  AppData        AppDataContent         `json:"appData"`
}
```

### Medium Priority (Feature Completeness):

1. **Add Missing Content-Specific Fields**:
```go
// NoteContent additions
type NoteContent struct {
  // ... existing fields ...
  EditorWidth         string `json:"editorWidth,omitempty"`
  AuthorizedForListed bool   `json:"authorizedForListed,omitempty"`
}

// TagContent additions
type TagContent struct {
  // ... existing fields ...
  Preferences interface{} `json:"preferences,omitempty"`
}

// ComponentContent additions
type ComponentContent struct {
  // ... existing fields ...
  IsDeprecated bool `json:"isDeprecated,omitempty"`
}
```

### Low Priority (Nice to Have):

1. **Improve Type Safety**:
   - Create proper Go structs for PackageInfo, Permissions
   - Consider time.Time for ValidUntil instead of string

2. **Enhance Documentation**:
   - Document which fields correspond to official Standard Notes features
   - Add field-level comments explaining usage

## Conclusion

gosn-v2 provides solid coverage of core Standard Notes item types (Note, Tag, Component) but has significant gaps in modern Standard Notes features. The analysis reveals:

### Coverage Summary:
- ✅ **Strong**: Core item types (Note, Tag, Component) with good attribute coverage
- ⚠️ **Partial**: File, SmartView, UserPreferences have type constants but incomplete content structures
- ❌ **Missing**: Modern collaboration features (TrustedContact, VaultListing, KeySystemRootKey, KeySystemItemsKey)
- ❌ **Incomplete**: Base ItemContent attributes affecting all item types

### Critical Impact Areas:
1. **User Experience**: Missing base attributes (pinned, archived, starred, locked, protected) limit core functionality
2. **Collaboration**: No support for TrustedContact and VaultListing prevents shared vault usage
3. **Modern Security**: Missing KeySystem types prevent advanced encryption features
4. **File Management**: Incomplete File content structure limits file handling capabilities

### Implementation Priority:
1. **High Priority**: Base ItemContent attributes + missing item types (essential for full compatibility)
2. **Medium Priority**: Complete existing partial implementations (File, SmartView, UserPrefs)
3. **Low Priority**: Enhanced content-specific attributes (nice-to-have features)

Implementing the High Priority recommendations would bring gosn-v2 to near-complete compatibility with the official Standard Notes app, enabling full feature parity for modern Standard Notes applications built with gosn-v2.
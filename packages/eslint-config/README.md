# `@auraedu/eslint-config`

AuraEDU's shared ESLint flat configuration for TypeScript packages. It combines ESLint's recommended rules with type-aware TypeScript rules and applies the repository's generated-file exclusions.

## Usage

```js
import auraedu from "@auraedu/eslint-config";

export default [...auraedu];
```

Packages may add narrowly scoped rules after the shared config. Do not disable correctness or security rules repository-wide to accommodate one file; fix the source or use a documented local exception.

export type Feature = { name: string; scopes: string[] };

// A feature is enabled when the token's space-separated scp covers all its scopes.
export function enabledFeatures<F extends Feature>(features: F[], scopes: string): F[] {
  const have = new Set(scopes.split(" ").filter(Boolean));
  return features.filter((f) => f.scopes.every((s) => have.has(s)));
}

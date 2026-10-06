import { useQuery } from "@tanstack/react-query";
import { enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery } from "./queries";

// Feature names as the backend registers them (Name in feature/*).
export const skillsFeature = "Skill Management";

// Characters whose token covers the feature, empty while loading.
export function useCharactersWithFeature(name: string) {
  const { data: characters = [] } = useQuery(charactersQuery);
  const { data: features = [] } = useQuery(featuresQuery);
  return characters.filter((c) => enabledFeatures(features, c.scopes).some((f) => f.name === name));
}

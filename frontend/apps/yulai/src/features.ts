import { useQuery } from "@tanstack/react-query";
import { enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery } from "./queries";

// Feature IDs as the backend registers them (ID in feature/*).
export const skillsFeature = "skills";
export const presenceFeature = "presence";

// The feature's picker name and the characters whose token covers it, no characters while loading.
export function useFeature(id: string) {
  const { data: characters = [] } = useQuery(charactersQuery);
  const { data: features = [] } = useQuery(featuresQuery);
  const feature = features.find((f) => f.id === id);
  return {
    name: feature?.name ?? id,
    characters: feature ? characters.filter((c) => enabledFeatures([feature], c.scopes).length > 0) : [],
  };
}

import { createContext, useContext } from "react";

export interface SearchContextValue {
  query: string;
  setQuery: (query: string) => void;
}

export const SearchContext = createContext<SearchContextValue>({ query: "", setQuery: () => {} });

export function useSearch(): SearchContextValue {
  return useContext(SearchContext);
}

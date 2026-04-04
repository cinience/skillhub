import { createContext, useContext } from 'react';
import type { User } from '../api/auth';

export interface AuthContextType {
  user: User | null;
  loading: boolean;
  refresh: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: true,
  refresh: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

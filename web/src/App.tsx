import { useEffect, useState, useCallback } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthContext } from './hooks/useAuth';
import { whoami, type User } from './api/auth';
import Layout from './components/Layout';
import Home from './pages/Home';
import Skills from './pages/Skills';
import SkillDetail from './pages/SkillDetail';
import Search from './pages/Search';
import Publish from './pages/Publish';
import Login from './pages/Login';

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    const u = await whoami();
    setUser(u);
    setLoading(false);
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  return (
    <AuthContext.Provider value={{ user, loading, refresh }}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Home />} />
            <Route path="/skills" element={<Skills />} />
            <Route path="/skills/:slug" element={<SkillDetail />} />
            <Route path="/search" element={<Search />} />
            <Route path="/publish" element={<Publish />} />
            <Route path="/login" element={<Login />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthContext.Provider>
  );
}

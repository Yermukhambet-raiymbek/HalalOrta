import { BrowserRouter, Route, Routes } from "react-router";
import Header from "./components/Header";
import FamilyPage from "./pages/FamilyPage";
import HomePage from "./pages/HomePage";
import JoinPage from "./pages/JoinPage";
import LearnPage from "./pages/LearnPage";
import LessonPage from "./pages/LessonPage";
import NotFoundPage from "./pages/NotFoundPage";
import { SessionProvider } from "./session";

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Header />
        <main className="page">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/join/:token" element={<JoinPage />} />
            <Route path="/family" element={<FamilyPage />} />
            <Route path="/learn" element={<LearnPage />} />
            <Route path="/learn/:slug" element={<LessonPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </main>
      </SessionProvider>
    </BrowserRouter>
  );
}

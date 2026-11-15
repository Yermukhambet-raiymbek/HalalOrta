import { BrowserRouter, Route, Routes } from "react-router";
import Footer from "./components/Footer";
import Header from "./components/Header";
import FamilyPage from "./pages/FamilyPage";
import HomePage from "./pages/HomePage";
import JoinPage from "./pages/JoinPage";
import LearnPage from "./pages/LearnPage";
import LessonPage from "./pages/LessonPage";
import NotFoundPage from "./pages/NotFoundPage";
import PricingPage from "./pages/PricingPage";
import TasksPage from "./pages/TasksPage";
import TodayPage from "./pages/TodayPage";
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
            <Route path="/today" element={<TodayPage />} />
            <Route path="/family" element={<FamilyPage />} />
            <Route path="/tasks" element={<TasksPage />} />
            <Route path="/learn" element={<LearnPage />} />
            <Route path="/learn/:slug" element={<LessonPage />} />
            <Route path="/pricing" element={<PricingPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </main>
        <Footer />
      </SessionProvider>
    </BrowserRouter>
  );
}

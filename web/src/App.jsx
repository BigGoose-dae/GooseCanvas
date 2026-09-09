import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import Projects from "./components/Projects";
import CanvasPage from "./components/CanvasPage";
import ModelSettings from "./components/ModelSettings";
import SystemSettings from "./components/SystemSettings";

export default function App() {
  const location = useLocation();
  return (
    <Routes>
      <Route path="/" element={<Projects />} />
      <Route
        path="/canvas/:id"
        element={<CanvasPage key={location.pathname} />}
      />
      <Route path="/settings" element={<SystemSettings />} />
      <Route path="/settings/models" element={<ModelSettings />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

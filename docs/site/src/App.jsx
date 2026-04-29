import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import Benchmarks from './pages/Benchmarks';
import Test from './pages/Test';

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Benchmarks />} />
        <Route path="/test" element={<Test />} />
      </Routes>
    </Layout>
  );
}

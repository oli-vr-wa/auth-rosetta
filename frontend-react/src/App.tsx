import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom' }

export default function App() { 
	return (
		<BrowserRouter>
			<Routes>
				<Route path="/register" element={<div>Register Page</div>} />
				<Route path="/login" element={<div>Login Page</div>} />
				<Route path="/success" element={<div>Success Page</div>} />
				<Route path="*" element={<Navigate to="/login" replace />} />
			</Routes>
		</BrowserRouter>
	);
}



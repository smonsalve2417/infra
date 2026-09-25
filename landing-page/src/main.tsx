import React from 'react';
import ReactDOM from 'react-dom';
import './CSS/index.css';  // Importa los estilos globales
import App from '../src/Pages/App';    // Importa tu componente App

// Montar la aplicación en el div con id 'root' del HTML
ReactDOM.render(
  <React.StrictMode>
    <App /> {/* Este es tu componente principal */}
  </React.StrictMode>,
  document.getElementById('root') // Esto se monta en el div id="root" en index.html
);

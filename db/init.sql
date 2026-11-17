CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    es_tutor BOOLEAN NOT NULL DEFAULT FALSE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice único insensible a mayúsculas/minúsculas y filtro rápido de tutores
CREATE UNIQUE INDEX IF NOT EXISTS usuarios_email_unico ON usuarios (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_usuarios_es_tutor ON usuarios (es_tutor) WHERE es_tutor = TRUE;

CREATE TABLE IF NOT EXISTS ramos (
    codigo TEXT PRIMARY KEY,
    nombre TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tutor_ramos (
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    ramo_codigo TEXT NOT NULL REFERENCES ramos(codigo) ON DELETE RESTRICT,
    nota_aprobacion NUMERIC(2,1) NOT NULL CHECK (nota_aprobacion >= 4.0 AND nota_aprobacion <= 7.0),
    periodo_aprobacion TEXT NOT NULL CHECK (periodo_aprobacion ~ '^[0-9]{4}-[12]$'),
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    verificado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (usuario_id, ramo_codigo)
);

CREATE INDEX IF NOT EXISTS idx_tutor_ramos_busqueda ON tutor_ramos (ramo_codigo, activo);

-- Catálogo inicial de ramos para pruebas y selectores del frontend
INSERT INTO ramos (codigo, nombre) VALUES
    ('MAT1610', 'Cálculo I'),
    ('MAT1620', 'Cálculo II'),
    ('IIC1103', 'Introducción a la Programación'),
    ('IIC2133', 'Estructuras de Datos y Algoritmos'),
    ('FIS1514', 'Dinámica')
ON CONFLICT (codigo) DO NOTHING;
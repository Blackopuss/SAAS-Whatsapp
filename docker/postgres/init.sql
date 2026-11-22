-- Dos usuarios: el migrador es dueño del esquema, la app solo toca datos (sin CREATE/DROP).
-- Contraseñas solo para desarrollo local; en producción vienen del entorno.
CREATE ROLE tiendas_migrator LOGIN PASSWORD 'migrator_dev';
CREATE ROLE tiendas_app LOGIN PASSWORD 'app_dev';

REVOKE ALL ON DATABASE tiendas FROM PUBLIC;
GRANT CONNECT ON DATABASE tiendas TO tiendas_migrator, tiendas_app;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
ALTER SCHEMA public OWNER TO tiendas_migrator;
GRANT USAGE ON SCHEMA public TO tiendas_app;

-- Todo lo que cree el migrador queda con permisos DML para la app.
ALTER DEFAULT PRIVILEGES FOR ROLE tiendas_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO tiendas_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tiendas_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO tiendas_app;

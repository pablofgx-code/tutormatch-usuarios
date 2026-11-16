package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tutormatch/usuarios/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CrearUsuario(ctx context.Context, u *domain.Usuario) error {
	query := `
		INSERT INTO usuarios (id, nombre, email, password_hash, es_tutor, creado_en)
		VALUES (gen_random_uuid(), $1, $2, $3, false, $4)
		RETURNING id, es_tutor
	`
	emailNormalizado := domain.NormalizarEmail(u.Email)

	err := r.db.QueryRowContext(ctx, query, u.Nombre, emailNormalizado, u.PasswordHash, u.CreadoEn).Scan(&u.ID, &u.EsTutor)
	if err != nil {
		if strings.Contains(err.Error(), "violates unique constraint") || strings.Contains(err.Error(), "usuarios_email_unico") {
			return domain.ErrYaExiste
		}
		return fmt.Errorf("error al crear usuario en db: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ObtenerPorEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	// Agregamos es_tutor al SELECT
	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE lower(email) = lower($1)
	`
	u := &domain.Usuario{}
	emailNormalizado := domain.NormalizarEmail(email)

	err := r.db.QueryRowContext(ctx, query, emailNormalizado).Scan(
		&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.EsTutor, &u.CreadoEn,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoEncontrado
		}
		return nil, fmt.Errorf("error al obtener usuario por email: %w", err)
	}

	ramos, err := r.obtenerRamosDeUsuario(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.RamosAprobados = ramos

	return u, nil
}

func (r *PostgresRepository) ObtenerPorID(ctx context.Context, id string) (*domain.Usuario, error) {
	// Agregamos es_tutor al SELECT
	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE id = $1
	`
	u := &domain.Usuario{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.EsTutor, &u.CreadoEn,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoEncontrado
		}
		return nil, fmt.Errorf("error al obtener usuario por id: %w", err)
	}

	ramos, err := r.obtenerRamosDeUsuario(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.RamosAprobados = ramos

	return u, nil
}

func (r *PostgresRepository) ObtenerPorIDs(ctx context.Context, ids []string) ([]*domain.Usuario, error) {
	if len(ids) == 0 {
		return []*domain.Usuario{}, nil
	}

	// Agregamos es_tutor al SELECT
	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE id = ANY($1)
	`
	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuarios en lote: %w", err)
	}
	defer rows.Close()

	var usuarios []*domain.Usuario
	for rows.Next() {
		u := &domain.Usuario{}
		// Escaneamos es_tutor
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.EsTutor, &u.CreadoEn); err != nil {
			return nil, err
		}
		ramos, _ := r.obtenerRamosDeUsuario(ctx, u.ID)
		u.RamosAprobados = ramos
		usuarios = append(usuarios, u)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando usuarios en lote: %w", err)
	}

	return usuarios, nil
}

func (r *PostgresRepository) ActualizarRamos(ctx context.Context, usuarioID string, ramoCodigos []string) (*domain.Usuario, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Limpiamos los ramos anteriores
	_, err = tx.ExecContext(ctx, `DELETE FROM usuario_ramos WHERE usuario_id = $1`, usuarioID)
	if err != nil {
		return nil, fmt.Errorf("error al limpiar ramos anteriores: %w", err)
	}

	// 2. Insertamos los nuevos ramos
	for _, codigo := range ramoCodigos {
		_, err = tx.ExecContext(ctx, `INSERT INTO usuario_ramos (usuario_id, ramo_codigo) VALUES ($1, $2)`, usuarioID, codigo)
		if err != nil {
			return nil, fmt.Errorf("error al insertar nuevo ramo (posible código inexistente): %w", err)
		}
	}

	// 3. RECALCULAMOS es_tutor automáticamente según si tiene ramos o no
	_, err = tx.ExecContext(ctx, `
		UPDATE usuarios 
		SET es_tutor = EXISTS (SELECT 1 FROM usuario_ramos WHERE usuario_id = $1)
		WHERE id = $1
	`, usuarioID)
	if err != nil {
		return nil, fmt.Errorf("error al recalcular es_tutor: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.ObtenerPorID(ctx, usuarioID)
}

func (r *PostgresRepository) BuscarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
	// Agregamos es_tutor al SELECT y filtramos eficientemente por u.es_tutor = true
	query := `
		SELECT u.id, u.nombre, u.email, u.password_hash, u.es_tutor, u.creado_en
		FROM usuarios u
		JOIN usuario_ramos ur ON u.id = ur.usuario_id
		WHERE ur.ramo_codigo = $1 AND u.es_tutor = true
	`
	rows, err := r.db.QueryContext(ctx, query, ramoCodigo)
	if err != nil {
		return nil, fmt.Errorf("error al buscar tutores por ramo: %w", err)
	}
	defer rows.Close()

	var tutores []*domain.Usuario
	for rows.Next() {
		u := &domain.Usuario{}
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.EsTutor, &u.CreadoEn); err != nil {
			return nil, err
		}
		ramos, _ := r.obtenerRamosDeUsuario(ctx, u.ID)
		u.RamosAprobados = ramos
		tutores = append(tutores, u)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando tutores por ramo: %w", err)
	}

	return tutores, nil
}

func (r *PostgresRepository) ListarRamos(ctx context.Context) ([]domain.Ramo, error) {
	query := `SELECT codigo, nombre FROM ramos`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al listar ramos: %w", err)
	}
	defer rows.Close()

	var ramos []domain.Ramo
	for rows.Next() {
		var rModel domain.Ramo
		if err := rows.Scan(&rModel.Codigo, &rModel.Nombre); err != nil {
			return nil, err
		}
		ramos = append(ramos, rModel)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando el catálogo de ramos: %w", err)
	}

	return ramos, nil
}

func (r *PostgresRepository) VerificarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, bool, error) {
	var existe bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usuarios WHERE id = $1)`, tutorID).Scan(&existe)
	if err != nil || !existe {
		return false, false, nil
	}

	var aprobo bool
	err = r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usuario_ramos WHERE usuario_id = $1 AND ramo_codigo = $2)`, tutorID, ramoCodigo).Scan(&aprobo)
	if err != nil {
		return true, false, err
	}

	return true, aprobo, nil
}

func (r *PostgresRepository) obtenerRamosDeUsuario(ctx context.Context, usuarioID string) ([]domain.Ramo, error) {
	query := `
		SELECT r.codigo, r.nombre
		FROM ramos r
		JOIN usuario_ramos ur ON r.codigo = ur.ramo_codigo
		WHERE ur.usuario_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ramos []domain.Ramo
	for rows.Next() {
		var rm domain.Ramo
		if err := rows.Scan(&rm.Codigo, &rm.Nombre); err != nil {
			return nil, err
		}
		ramos = append(ramos, rm)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando ramos del usuario: %w", err)
	}

	return ramos, nil
}

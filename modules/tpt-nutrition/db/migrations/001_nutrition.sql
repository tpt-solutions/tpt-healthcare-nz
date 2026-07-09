-- Nutrition food diary entries.
CREATE TABLE IF NOT EXISTS nutrition_food_diary (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    date            BIGINT NOT NULL DEFAULT 0,
    meal_type       TEXT NOT NULL DEFAULT '',
    food_item       TEXT NOT NULL DEFAULT '',
    portion         TEXT NOT NULL DEFAULT '',
    calories        INT NOT NULL DEFAULT 0,
    protein_g       NUMERIC(8,2) NOT NULL DEFAULT 0,
    carbs_g         NUMERIC(8,2) NOT NULL DEFAULT 0,
    fat_g           NUMERIC(8,2) NOT NULL DEFAULT 0,
    fiber_g         NUMERIC(8,2) NOT NULL DEFAULT 0,
    sugar_g         NUMERIC(8,2) NOT NULL DEFAULT 0,
    sodium_mg       NUMERIC(8,2) NOT NULL DEFAULT 0,
    notes           TEXT NOT NULL DEFAULT '',
    clinician_id    TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_nutrition_diary_patient ON nutrition_food_diary(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_nutrition_diary_date ON nutrition_food_diary(date);

-- Nutrition meal plans.
CREATE TABLE IF NOT EXISTS nutrition_meal_plans (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    practice_id     TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    goal            TEXT NOT NULL DEFAULT '',
    calorie_target  INT NOT NULL DEFAULT 0,
    duration_days   INT NOT NULL DEFAULT 0,
    meals           JSONB NOT NULL DEFAULT '[]',
    restrictions    TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_nutrition_plan_patient ON nutrition_meal_plans(patient_nhi);

-- Nutrition body composition measurements.
CREATE TABLE IF NOT EXISTS nutrition_body_comp (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    date            BIGINT NOT NULL DEFAULT 0,
    weight_kg       NUMERIC(8,2) NOT NULL DEFAULT 0,
    height_cm       NUMERIC(8,2) NOT NULL DEFAULT 0,
    bmi             NUMERIC(8,2) NOT NULL DEFAULT 0,
    body_fat_pct    NUMERIC(8,2) NOT NULL DEFAULT 0,
    muscle_mass     NUMERIC(8,2) NOT NULL DEFAULT 0,
    bone_mass       NUMERIC(8,2) NOT NULL DEFAULT 0,
    body_water_pct  NUMERIC(8,2) NOT NULL DEFAULT 0,
    waist_cm        NUMERIC(8,2) NOT NULL DEFAULT 0,
    hip_cm          NUMERIC(8,2) NOT NULL DEFAULT 0,
    whr             NUMERIC(8,2) NOT NULL DEFAULT 0,
    visceral_fat    INT NOT NULL DEFAULT 0,
    bmr             INT NOT NULL DEFAULT 0,
    metabolic_age   INT NOT NULL DEFAULT 0,
    clinician_id    TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_nutrition_bc_patient ON nutrition_body_comp(patient_nhi);

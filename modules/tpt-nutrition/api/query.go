package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/bodycomp"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/fooddiary"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/mealplan"
)

var errNotFound = errors.New("record not found")

// ---- Food Diary ----

const diarySelectCols = `id, patient_nhi, date, meal_type, food_item, portion,
	calories, protein_g, carbs_g, fat_g, fiber_g, sugar_g, sodium_mg,
	notes, clinician_id, created_at, updated_at`

func (s *Server) listDiaryEntries(ctx context.Context, patientNHI string) ([]fooddiary.FoodDiaryEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+diarySelectCols+`
		 FROM nutrition_food_diary
		 WHERE patient_nhi = @nhi
		 ORDER BY date DESC, created_at DESC
		 LIMIT 200`,
		db.NamedArgs{"nhi": patientNHI},
	)
	if err != nil {
		return nil, fmt.Errorf("query food diary: %w", err)
	}
	defer rows.Close()

	var results []fooddiary.FoodDiaryEntry
	for rows.Next() {
		var e fooddiary.FoodDiaryEntry
		if err := rows.Scan(
			&e.ID, &e.PatientNHI, &e.Date, &e.MealType, &e.FoodItem, &e.Portion,
			&e.Calories, &e.ProteinG, &e.CarbsG, &e.FatG, &e.FiberG, &e.SugarG, &e.SodiumMg,
			&e.Notes, &e.ClinicianID, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan food diary entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (s *Server) getDiaryEntry(ctx context.Context, id, patientNHI string) (fooddiary.FoodDiaryEntry, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+diarySelectCols+`
		 FROM nutrition_food_diary
		 WHERE id = @id AND patient_nhi = @nhi`,
		db.NamedArgs{"id": id, "nhi": patientNHI},
	)
	var e fooddiary.FoodDiaryEntry
	if err := row.Scan(
		&e.ID, &e.PatientNHI, &e.Date, &e.MealType, &e.FoodItem, &e.Portion,
		&e.Calories, &e.ProteinG, &e.CarbsG, &e.FatG, &e.FiberG, &e.SugarG, &e.SodiumMg,
		&e.Notes, &e.ClinicianID, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return fooddiary.FoodDiaryEntry{}, errNotFound
		}
		return fooddiary.FoodDiaryEntry{}, fmt.Errorf("get food diary entry: %w", err)
	}
	return e, nil
}

func (s *Server) createDiaryEntry(ctx context.Context, e fooddiary.FoodDiaryEntry) (fooddiary.FoodDiaryEntry, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO nutrition_food_diary
		   (id, patient_nhi, date, meal_type, food_item, portion,
		    calories, protein_g, carbs_g, fat_g, fiber_g, sugar_g, sodium_mg,
		    notes, clinician_id, created_at, updated_at)
		 VALUES
		   (@id, @patient_nhi, @date, @meal_type, @food_item, @portion,
		    @calories, @protein_g, @carbs_g, @fat_g, @fiber_g, @sugar_g, @sodium_mg,
		    @notes, @clinician_id, @created_at, @updated_at)
		 RETURNING `+diarySelectCols,
		db.NamedArgs{
			"id":           e.ID,
			"patient_nhi":  e.PatientNHI,
			"date":         e.Date,
			"meal_type":    e.MealType,
			"food_item":    e.FoodItem,
			"portion":      e.Portion,
			"calories":     e.Calories,
			"protein_g":    e.ProteinG,
			"carbs_g":      e.CarbsG,
			"fat_g":        e.FatG,
			"fiber_g":      e.FiberG,
			"sugar_g":      e.SugarG,
			"sodium_mg":    e.SodiumMg,
			"notes":        e.Notes,
			"clinician_id": e.ClinicianID,
			"created_at":   e.CreatedAt,
			"updated_at":   e.UpdatedAt,
		},
	)
	var result fooddiary.FoodDiaryEntry
	if err := row.Scan(
		&result.ID, &result.PatientNHI, &result.Date, &result.MealType, &result.FoodItem, &result.Portion,
		&result.Calories, &result.ProteinG, &result.CarbsG, &result.FatG, &result.FiberG, &result.SugarG, &result.SodiumMg,
		&result.Notes, &result.ClinicianID, &result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return fooddiary.FoodDiaryEntry{}, fmt.Errorf("insert food diary entry: %w", err)
	}
	return result, nil
}

func (s *Server) updateDiaryEntry(ctx context.Context, e fooddiary.FoodDiaryEntry) (fooddiary.FoodDiaryEntry, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE nutrition_food_diary
		 SET date         = @date,
		     meal_type    = @meal_type,
		     food_item    = @food_item,
		     portion      = @portion,
		     calories     = @calories,
		     protein_g    = @protein_g,
		     carbs_g      = @carbs_g,
		     fat_g        = @fat_g,
		     fiber_g      = @fiber_g,
		     sugar_g      = @sugar_g,
		     sodium_mg    = @sodium_mg,
		     notes        = @notes,
		     clinician_id = @clinician_id,
		     updated_at   = @updated_at
		 WHERE id = @id AND patient_nhi = @patient_nhi
		 RETURNING `+diarySelectCols,
		db.NamedArgs{
			"id":           e.ID,
			"patient_nhi":  e.PatientNHI,
			"date":         e.Date,
			"meal_type":    e.MealType,
			"food_item":    e.FoodItem,
			"portion":      e.Portion,
			"calories":     e.Calories,
			"protein_g":    e.ProteinG,
			"carbs_g":      e.CarbsG,
			"fat_g":        e.FatG,
			"fiber_g":      e.FiberG,
			"sugar_g":      e.SugarG,
			"sodium_mg":    e.SodiumMg,
			"notes":        e.Notes,
			"clinician_id": e.ClinicianID,
			"updated_at":   e.UpdatedAt,
		},
	)
	var result fooddiary.FoodDiaryEntry
	if err := row.Scan(
		&result.ID, &result.PatientNHI, &result.Date, &result.MealType, &result.FoodItem, &result.Portion,
		&result.Calories, &result.ProteinG, &result.CarbsG, &result.FatG, &result.FiberG, &result.SugarG, &result.SodiumMg,
		&result.Notes, &result.ClinicianID, &result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return fooddiary.FoodDiaryEntry{}, errNotFound
		}
		return fooddiary.FoodDiaryEntry{}, fmt.Errorf("update food diary entry: %w", err)
	}
	return result, nil
}

// ---- Meal Plans ----

const planSelectCols = `id, patient_nhi, clinician_id, practice_id, name, goal,
	calorie_target, duration_days, meals, restrictions, notes, active,
	created_at, updated_at`

func (s *Server) listMealPlans(ctx context.Context, patientNHI string) ([]mealplan.MealPlan, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+planSelectCols+`
		 FROM nutrition_meal_plans
		 WHERE patient_nhi = @nhi
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{"nhi": patientNHI},
	)
	if err != nil {
		return nil, fmt.Errorf("query meal plans: %w", err)
	}
	defer rows.Close()

	var results []mealplan.MealPlan
	for rows.Next() {
		var p mealplan.MealPlan
		var mealsJSON []byte
		if err := rows.Scan(
			&p.ID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID, &p.Name, &p.Goal,
			&p.CalorieTarget, &p.DurationDays, &mealsJSON, &p.Restrictions, &p.Notes, &p.Active,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan meal plan: %w", err)
		}
		if mealsJSON != nil {
			if err := json.Unmarshal(mealsJSON, &p.Meals); err != nil {
				return nil, fmt.Errorf("unmarshal meal plan meals: %w", err)
			}
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (s *Server) getMealPlan(ctx context.Context, id, patientNHI string) (mealplan.MealPlan, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+planSelectCols+`
		 FROM nutrition_meal_plans
		 WHERE id = @id AND patient_nhi = @nhi`,
		db.NamedArgs{"id": id, "nhi": patientNHI},
	)
	var p mealplan.MealPlan
	var mealsJSON []byte
	if err := row.Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID, &p.Name, &p.Goal,
		&p.CalorieTarget, &p.DurationDays, &mealsJSON, &p.Restrictions, &p.Notes, &p.Active,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return mealplan.MealPlan{}, errNotFound
		}
		return mealplan.MealPlan{}, fmt.Errorf("get meal plan: %w", err)
	}
	if mealsJSON != nil {
		if err := json.Unmarshal(mealsJSON, &p.Meals); err != nil {
			return mealplan.MealPlan{}, fmt.Errorf("unmarshal meal plan meals: %w", err)
		}
	}
	return p, nil
}

func (s *Server) createMealPlan(ctx context.Context, p mealplan.MealPlan) (mealplan.MealPlan, error) {
	mealsJSON, err := json.Marshal(p.Meals)
	if err != nil {
		return mealplan.MealPlan{}, fmt.Errorf("marshal meal plan meals: %w", err)
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO nutrition_meal_plans
		   (id, patient_nhi, clinician_id, practice_id, name, goal,
		    calorie_target, duration_days, meals, restrictions, notes, active,
		    created_at, updated_at)
		 VALUES
		   (@id, @patient_nhi, @clinician_id, @practice_id, @name, @goal,
		    @calorie_target, @duration_days, @meals::jsonb, @restrictions, @notes, @active,
		    @created_at, @updated_at)
		 RETURNING `+planSelectCols,
		db.NamedArgs{
			"id":             p.ID,
			"patient_nhi":    p.PatientNHI,
			"clinician_id":   p.ClinicianID,
			"practice_id":    p.PracticeID,
			"name":           p.Name,
			"goal":           p.Goal,
			"calorie_target": p.CalorieTarget,
			"duration_days":  p.DurationDays,
			"meals":          mealsJSON,
			"restrictions":   p.Restrictions,
			"notes":          p.Notes,
			"active":         p.Active,
			"created_at":     p.CreatedAt,
			"updated_at":     p.UpdatedAt,
		},
	)
	var result mealplan.MealPlan
	var resultMealsJSON []byte
	if err := row.Scan(
		&result.ID, &result.PatientNHI, &result.ClinicianID, &result.PracticeID, &result.Name, &result.Goal,
		&result.CalorieTarget, &result.DurationDays, &resultMealsJSON, &result.Restrictions, &result.Notes, &result.Active,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return mealplan.MealPlan{}, fmt.Errorf("insert meal plan: %w", err)
	}
	if resultMealsJSON != nil {
		if err := json.Unmarshal(resultMealsJSON, &result.Meals); err != nil {
			return mealplan.MealPlan{}, fmt.Errorf("unmarshal inserted meal plan meals: %w", err)
		}
	}
	return result, nil
}

func (s *Server) updateMealPlan(ctx context.Context, p mealplan.MealPlan) (mealplan.MealPlan, error) {
	mealsJSON, err := json.Marshal(p.Meals)
	if err != nil {
		return mealplan.MealPlan{}, fmt.Errorf("marshal meal plan meals: %w", err)
	}
	row := s.pool.QueryRow(ctx,
		`UPDATE nutrition_meal_plans
		 SET clinician_id   = @clinician_id,
		     practice_id    = @practice_id,
		     name           = @name,
		     goal           = @goal,
		     calorie_target = @calorie_target,
		     duration_days  = @duration_days,
		     meals          = @meals::jsonb,
		     restrictions   = @restrictions,
		     notes          = @notes,
		     active         = @active,
		     updated_at     = @updated_at
		 WHERE id = @id AND patient_nhi = @patient_nhi
		 RETURNING `+planSelectCols,
		db.NamedArgs{
			"id":             p.ID,
			"patient_nhi":    p.PatientNHI,
			"clinician_id":   p.ClinicianID,
			"practice_id":    p.PracticeID,
			"name":           p.Name,
			"goal":           p.Goal,
			"calorie_target": p.CalorieTarget,
			"duration_days":  p.DurationDays,
			"meals":          mealsJSON,
			"restrictions":   p.Restrictions,
			"notes":          p.Notes,
			"active":         p.Active,
			"updated_at":     p.UpdatedAt,
		},
	)
	var result mealplan.MealPlan
	var resultMealsJSON []byte
	if err := row.Scan(
		&result.ID, &result.PatientNHI, &result.ClinicianID, &result.PracticeID, &result.Name, &result.Goal,
		&result.CalorieTarget, &result.DurationDays, &resultMealsJSON, &result.Restrictions, &result.Notes, &result.Active,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return mealplan.MealPlan{}, errNotFound
		}
		return mealplan.MealPlan{}, fmt.Errorf("update meal plan: %w", err)
	}
	if resultMealsJSON != nil {
		if err := json.Unmarshal(resultMealsJSON, &result.Meals); err != nil {
			return mealplan.MealPlan{}, fmt.Errorf("unmarshal updated meal plan meals: %w", err)
		}
	}
	return result, nil
}

// ---- Body Composition ----

const bodyCompSelectCols = `id, patient_nhi, date, weight_kg, height_cm, bmi,
	body_fat_pct, muscle_mass, bone_mass, body_water_pct, waist_cm, hip_cm,
	whr, visceral_fat, bmr, metabolic_age, clinician_id, notes, created_at, updated_at`

func (s *Server) getLatestBodyComp(ctx context.Context, patientNHI string) (bodycomp.BodyComposition, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+bodyCompSelectCols+`
		 FROM nutrition_body_comp
		 WHERE patient_nhi = @nhi
		 ORDER BY date DESC, created_at DESC
		 LIMIT 1`,
		db.NamedArgs{"nhi": patientNHI},
	)
	var bc bodycomp.BodyComposition
	if err := row.Scan(
		&bc.ID, &bc.PatientNHI, &bc.Date, &bc.WeightKg, &bc.HeightCm, &bc.BMI,
		&bc.BodyFatPct, &bc.MuscleMass, &bc.BoneMass, &bc.BodyWaterPct, &bc.WaistCm, &bc.HipCm,
		&bc.WHR, &bc.VisceralFat, &bc.BMR, &bc.MetabolicAge, &bc.ClinicianID, &bc.Notes,
		&bc.CreatedAt, &bc.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return bodycomp.BodyComposition{}, errNotFound
		}
		return bodycomp.BodyComposition{}, fmt.Errorf("get body composition: %w", err)
	}
	return bc, nil
}

func (s *Server) createBodyComp(ctx context.Context, bc bodycomp.BodyComposition) (bodycomp.BodyComposition, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO nutrition_body_comp
		   (id, patient_nhi, date, weight_kg, height_cm, bmi,
		    body_fat_pct, muscle_mass, bone_mass, body_water_pct, waist_cm, hip_cm,
		    whr, visceral_fat, bmr, metabolic_age, clinician_id, notes, created_at, updated_at)
		 VALUES
		   (@id, @patient_nhi, @date, @weight_kg, @height_cm, @bmi,
		    @body_fat_pct, @muscle_mass, @bone_mass, @body_water_pct, @waist_cm, @hip_cm,
		    @whr, @visceral_fat, @bmr, @metabolic_age, @clinician_id, @notes, @created_at, @updated_at)
		 RETURNING `+bodyCompSelectCols,
		db.NamedArgs{
			"id":            bc.ID,
			"patient_nhi":   bc.PatientNHI,
			"date":          bc.Date,
			"weight_kg":     bc.WeightKg,
			"height_cm":     bc.HeightCm,
			"bmi":           bc.BMI,
			"body_fat_pct":  bc.BodyFatPct,
			"muscle_mass":   bc.MuscleMass,
			"bone_mass":     bc.BoneMass,
			"body_water_pct": bc.BodyWaterPct,
			"waist_cm":      bc.WaistCm,
			"hip_cm":        bc.HipCm,
			"whr":           bc.WHR,
			"visceral_fat":  bc.VisceralFat,
			"bmr":           bc.BMR,
			"metabolic_age": bc.MetabolicAge,
			"clinician_id":  bc.ClinicianID,
			"notes":         bc.Notes,
			"created_at":    bc.CreatedAt,
			"updated_at":    bc.UpdatedAt,
		},
	)
	var result bodycomp.BodyComposition
	if err := row.Scan(
		&result.ID, &result.PatientNHI, &result.Date, &result.WeightKg, &result.HeightCm, &result.BMI,
		&result.BodyFatPct, &result.MuscleMass, &result.BoneMass, &result.BodyWaterPct, &result.WaistCm, &result.HipCm,
		&result.WHR, &result.VisceralFat, &result.BMR, &result.MetabolicAge, &result.ClinicianID, &result.Notes,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return bodycomp.BodyComposition{}, fmt.Errorf("insert body composition: %w", err)
	}
	return result, nil
}

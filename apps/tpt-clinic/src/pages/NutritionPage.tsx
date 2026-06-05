import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'food-diary' | 'meal-plans' | 'body-composition';

interface DiaryEntry { id: string; patientNhi: string; mealType: string; createdAt: number }
interface MealPlan { id: string; patientNhi: string; title: string; durationWeeks: number; createdAt: number }
interface BodyComp { id: string; patientNhi: string; weightKg: number; bmi: number; createdAt: number }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'], ['food-diary', 'Food Diary'], ['meal-plans', 'Meal Plans'], ['body-composition', 'Body Composition'],
  ];
  return (
    <div className="mb-6 flex gap-1 rounded-lg bg-secondary-100 p-1">
      {tabs.map(([id, label]) => (
        <button key={id} onClick={() => onSelect(id)}
          className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === id ? 'bg-white text-primary-700 shadow-sm' : 'text-secondary-600 hover:text-secondary-900'
          }`}>
          {label}
        </button>
      ))}
    </div>
  );
}

export default function NutritionPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [diaryEntries] = useState<DiaryEntry[]>([]);
  const [mealPlans, setMealPlans] = useState<MealPlan[]>([]);
  const [bodyComps] = useState<BodyComp[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      fetch('/api/v1/meal-plans').then(r => r.ok ? r.json() : []),
    ])
      .then(([mp]) => { setMealPlans(mp ?? []); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Nutrition">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Diary Entries', value: diaryEntries.length, border: 'border-primary-200' },
              { label: 'Meal Plans', value: mealPlans.length, border: 'border-green-200' },
              { label: 'Body Comp Records', value: bodyComps.length, border: 'border-amber-200' },
              { label: 'Active Plans', value: mealPlans.length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Clinical Nutrition</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Track patient food diaries, create personalised meal plans, and monitor body
              composition including BMI, body fat percentage, and waist-to-hip ratio.
              Supports private pay and some ACC-funded rehabilitation nutritional support.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('food-diary')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                Add Diary Entry
              </button>
              <button onClick={() => setActiveTab('meal-plans')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                Create Meal Plan
              </button>
              <button onClick={() => setActiveTab('body-composition')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                Record Measurements
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'food-diary' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Food Diary</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Add Entry
            </button>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {['Breakfast', 'Morning Snack', 'Lunch', 'Afternoon Snack', 'Dinner', 'Evening Snack'].map(meal => (
                <div key={meal} className="rounded-lg border border-secondary-200 p-3">
                  <p className="text-xs font-semibold uppercase text-secondary-500">{meal}</p>
                  <textarea rows={2} placeholder="Describe foods consumed…"
                    className="mt-2 w-full resize-none rounded-md border border-secondary-200 px-2 py-1.5 text-sm focus:border-primary-500 focus:outline-none" />
                  <div className="mt-1 flex gap-2">
                    <input type="number" placeholder="kJ" className="w-20 rounded border border-secondary-200 px-2 py-1 text-xs" />
                    <span className="text-xs text-secondary-400 self-center">kilojoules</span>
                  </div>
                </div>
              ))}
            </div>
            <button className="mt-4 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
              Save Diary Entry
            </button>
          </div>
        </div>
      )}

      {activeTab === 'meal-plans' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Meal Plans</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Create Plan
            </button>
          </div>
          {loading ? (
            <div className="flex h-32 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
            </div>
          ) : mealPlans.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No meal plans created.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Plan Title</th>
                  <th className="px-6 py-3 text-left">Duration</th>
                  <th className="px-6 py-3 text-left">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {mealPlans.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                    <td className="px-6 py-3 font-medium">{p.title}</td>
                    <td className="px-6 py-3">{p.durationWeeks} weeks</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(p.createdAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'body-composition' && (
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="mb-4 text-base font-semibold text-secondary-900">Body Composition Measurements</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
            {[
              { label: 'Weight (kg)', placeholder: '72.5' },
              { label: 'Height (cm)', placeholder: '168' },
              { label: 'BMI (calculated)', placeholder: 'Auto' },
              { label: 'Waist (cm)', placeholder: '80' },
              { label: 'Hip (cm)', placeholder: '95' },
              { label: 'Waist:Hip Ratio', placeholder: 'Auto' },
              { label: 'Body Fat %', placeholder: '22.5' },
              { label: 'Muscle Mass (kg)', placeholder: '45.2' },
              { label: 'Visceral Fat Level', placeholder: '1–12' },
            ].map(({ label, placeholder }) => (
              <div key={label}>
                <label className="block text-sm font-medium text-secondary-700">{label}</label>
                <input type="text" placeholder={placeholder}
                  className="mt-1 w-full rounded-md border border-secondary-300 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500" />
              </div>
            ))}
          </div>
          {bodyComps.length > 0 && (
            <div className="mt-6">
              <h3 className="mb-2 text-sm font-medium text-secondary-700">Previous Measurements</h3>
              <table className="w-full text-sm">
                <thead className="text-xs font-medium text-secondary-500">
                  <tr>
                    <th className="py-2 text-left">Date</th>
                    <th className="py-2 text-left">Weight (kg)</th>
                    <th className="py-2 text-left">BMI</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {bodyComps.map(b => (
                    <tr key={b.id}>
                      <td className="py-2 text-secondary-500">{new Date(b.createdAt).toLocaleDateString('en-NZ')}</td>
                      <td className="py-2">{b.weightKg}</td>
                      <td className="py-2">{b.bmi.toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <button className="mt-6 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
            Save Measurements
          </button>
        </div>
      )}
    </AppShell>
  );
}

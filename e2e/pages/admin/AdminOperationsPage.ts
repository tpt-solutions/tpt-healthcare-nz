import type { Page } from '@playwright/test';

export class AdminRosterPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Staff Roster' });
  }

  addShiftButton() {
    return this.page.getByRole('button', { name: 'Add shift' });
  }

  emptyState() {
    return this.page.getByText('No shifts scheduled');
  }
}

export class AdminRoomsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Room Bookings' });
  }

  bookRoomButton() {
    return this.page.getByRole('button', { name: 'Book room' });
  }

  emptyState() {
    return this.page.getByText('No room bookings found');
  }
}

export class AdminLeavePage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Leave Requests' });
  }

  requestLeaveButton() {
    return this.page.getByRole('button', { name: 'Request leave' });
  }

  emptyState() {
    return this.page.getByText('No leave requests');
  }

  approveButton() {
    return this.page.getByRole('button', { name: 'Approve' }).first();
  }

  declineButton() {
    return this.page.getByRole('button', { name: 'Decline' }).first();
  }
}

export class AdminInvoicesPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Patient Invoices' });
  }

  newInvoiceButton() {
    return this.page.getByRole('button', { name: 'New invoice' });
  }

  filterButton(label: string) {
    return this.page.getByRole('button', { name: new RegExp(`^${label}$`, 'i') });
  }
}

export class AdminInventoryPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Inventory' });
  }

  addStockItemButton() {
    return this.page.getByRole('button', { name: 'Add stock item' });
  }

  lowStockAlert() {
    return this.page.getByText(/below reorder point/);
  }

  emptyState() {
    return this.page.getByText('No stock items');
  }
}

export class AdminBudgetPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Budget Variance' });
  }

  totalPlannedCard() {
    return this.page.getByText('Total planned').locator('..');
  }

  varianceCard() {
    return this.page.getByText('Variance').locator('..');
  }
}

export class AdminDepartmentsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Departments' });
  }

  addDepartmentButton() {
    return this.page.getByRole('button', { name: 'Add department' });
  }

  emptyState() {
    return this.page.getByText('No departments configured');
  }

  nameInput() {
    return this.page.locator('input[placeholder="General Practice"]');
  }

  codeInput() {
    return this.page.locator('input[placeholder="gp"]');
  }

  saveButton() {
    return this.page.getByRole('button', { name: 'Save' }).first();
  }

  cancelButton() {
    return this.page.getByRole('button', { name: 'Cancel' }).first();
  }
}

export class AdminRolesPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Role Assignments' });
  }

  grantRoleButton() {
    return this.page.getByRole('button', { name: 'Grant role' });
  }

  emptyState() {
    return this.page.getByText('No role assignments yet');
  }

  userIdInput() {
    return this.page.getByPlaceholder('auth0|abc123 or hpi:99-ZZZ-99');
  }

  roleSelect() {
    return this.page.locator('select').first();
  }

  roleLegend() {
    return this.page.getByText('Role reference');
  }

  revokeButton() {
    return this.page.getByRole('button', { name: 'Revoke' }).first();
  }
}

export class AdminIntegrationsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Integrations' });
  }

  refreshButton() {
    return this.page.getByRole('button', { name: 'Refresh status' });
  }

  emptyState() {
    return this.page.getByText('No providers configured');
  }
}

export class AdminACCProviderPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'ACC Provider Registration' });
  }

  providerNumberInput() {
    return this.page.getByLabel('ACC Provider Number');
  }

  verifyButton() {
    return this.page.getByRole('button', { name: /Verify with ACC|Verifying/ });
  }

  saveProviderButton() {
    return this.page.getByRole('button', { name: 'Save Provider Number' });
  }

  helpSection() {
    return this.page.getByText('Getting your ACC provider number');
  }

  verificationResult() {
    return this.page.getByText('Verification result');
  }
}

export class AdminOnboardingPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Welcome to tpt-healthcare' });
  }

  stepIndicator() {
    return this.page.getByText(/Step \d+ of \d+/);
  }

  continueButton() {
    return this.page.getByRole('button', { name: 'Continue' });
  }

  backButton() {
    return this.page.getByRole('button', { name: 'Back' });
  }

  launchButton() {
    return this.page.getByRole('button', { name: 'Launch' });
  }
}

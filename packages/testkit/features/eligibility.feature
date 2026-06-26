Feature: Eligibility Verification
  Scenario: A temple verifies an adventurer's coverage before treatment
    Given an active adventurer created by the scenario
    And a Diamond-tier provider "Temple of the Healer, Vitesse"
    When the temple sends an eligibility inquiry
    Then a 270 transaction is dispatched
    And a 271 response is returned confirming active coverage

